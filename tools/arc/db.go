package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "turso.tech/database/tursogo"
)

const arcSchemaVersion = 1
const arcDBName = ".arc/arc.db"
const arcManifestName = ".arc/arc.json"
const legacyVerifyDB = ".arc/verify/story.db"

// arcManifest 描述 .arc/arc.json 中的统一数据库激活状态清单
type arcManifest struct {
	// Version 状态库Schema版本号
	Version int `json:"schema_version"`
	// Active 统一数据库是否已激活
	Active bool `json:"active"`
	// DB 激活数据库相对路径
	DB string `json:"db"`
	// LegacyDB 迁移来源旧库相对路径
	LegacyDB string `json:"legacy_db,omitempty"`
	// LegacySHA256 迁移前旧库SHA256校验和
	LegacySHA256 string `json:"legacy_sha256,omitempty"`
	// MigrationTime 迁移完成UTC时间
	MigrationTime string `json:"migrated_at,omitempty"`
	// ParityGraphs 等价性验证通过的图数量
	ParityGraphs int `json:"parity_graphs"`
}

// statePath 拼接仓库根目录下的统一状态数据库路径
// 请求体
// root 仓库根目录路径
// 响应体
// path .arc/arc.db 完整路径
func statePath(root string) string {
	// 1. 组合仓库根目录与固定数据库相对路径
	return filepath.Join(root, arcDBName)
}

// manifestPath 拼接仓库根目录下的切换清单路径
// 请求体
// root 仓库根目录路径
// 响应体
// path .arc/arc.json 完整路径
func manifestPath(root string) string {
	// 1. 组合仓库根目录与固定清单相对路径
	return filepath.Join(root, arcManifestName)
}

// openArcDB 打开单连接Turso数据库并配置连通性与锁超时
// 请求体
// path 数据库文件路径
// 响应体
// db 就绪的数据库连接句柄
// err 打开连接、Ping或配置PRAGMA失败错误
func openArcDB(path string) (*sql.DB, error) {
	// 1. 打开单连接池模式的Turso句柄
	db, err := sql.Open("turso", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	// 2. 在15秒超时内完成连通性探测与busy_timeout设置
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// openActiveArcDB 校验切换清单处于激活态后打开统一状态数据库
// 请求体
// root 仓库根目录路径
// 响应体
// db 激活态数据库连接句柄
// err 未激活或打开失败错误
func openActiveArcDB(root string) (*sql.DB, error) {
	// 1. 校验 .arc/arc.json 清单已激活
	if m, err := readManifest(root); err != nil {
		return nil, err
	} else if m == nil {
		return nil, errors.New("arc.db not active; use arc db init or explicit arc db migrate")
	}
	// 2. 打开并返回统一状态数据库连接
	return openArcDB(statePath(root))
}

// createArcSchema 在事务内创建统一资源表结构并校验Schema版本
// 请求体
// db 数据库连接句柄
// 响应体
// err 建表或Schema版本不匹配错误
func createArcSchema(db *sql.DB) error {
	// 1. 在45秒事务内依次执行全部DDL建表与索引语句
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	return dbTransaction(ctx, db, func(tx *sql.Tx) error {
		for _, stmt := range arcSchema {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("arc schema: %w", err)
			}
		}
		// 2. 写入并核对 schema_version 元数据一致性
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO arc_meta(key,value) VALUES('schema_version',?)", fmt.Sprint(arcSchemaVersion)); err != nil {
			return err
		}
		var v string
		if err := tx.QueryRowContext(ctx, "SELECT value FROM arc_meta WHERE key='schema_version'").Scan(&v); err != nil {
			return err
		}
		if v != fmt.Sprint(arcSchemaVersion) {
			return fmt.Errorf("arc schema version mismatch: %s", v)
		}
		return nil
	})
}

// isArcDBArgument 判断转发给子引擎的参数列表是否已显式指定 --db
// 请求体
// args 子命令参数切片
// 响应体
// present 是否已包含 --db 参数
func isArcDBArgument(args []string) bool {
	// 1. 扫描独立标志或等号拼接形式的 --db 参数
	for i, a := range args {
		if a == "--db" && i+1 < len(args) {
			return true
		}
		if strings.HasPrefix(a, "--db=") {
			return true
		}
	}
	return false
}

// dbCommand 执行 arc db status/init/migrate 数据库生命周期管理命令
// 请求体
// args 子命令参数切片
// 响应体
// err 参数解析、锁竞争或迁移一致性校验失败错误
func dbCommand(args []string) error {
	// 1. 解析动作名称与命令行标志并定位仓库根目录
	if len(args) == 0 {
		return errors.New("usage: arc db <status|init|migrate> [--repo DIR] [--confirm-offline]")
	}
	action := args[0]
	fs := flag.NewFlagSet("arc db "+action, flag.ContinueOnError)
	rootFlag := fs.String("repo", ".", "repository directory")
	confirm := fs.Bool("confirm-offline", false, "assert all legacy verifier processes are stopped")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("unexpected arguments %v", fs.Args())
	}
	root, err := repositoryRoot(*rootFlag)
	if err != nil {
		return err
	}
	// 2. 分发查询状态或初始化/离线迁移分支
	switch action {
	case "status":
		return executeDBStatus(root)
	case "init", "migrate":
		return executeDBInitOrMigrate(root, action, *confirm)
	default:
		return fmt.Errorf("unknown arc db action %q", action)
	}
}

// executeDBStatus 查询并输出当前仓库 arc.db 的具名状态响应
// 请求体
// root 仓库根目录路径
// 响应体
// err 读取清单或查询统计信息错误
func executeDBStatus(root string) error {
	// 1. 读取切换清单构建基础状态响应结构
	m, err := readManifest(root)
	if err != nil {
		return err
	}
	resp := dbStatusResponse{
		Path:          statePath(root),
		Active:        m != nil,
		SchemaVersion: arcSchemaVersion,
	}
	// 2. 若已激活则在仓库锁内补充待处理队列、图计数与策略版本指标
	if m != nil {
		parity := m.ParityGraphs
		resp.LegacyDB = m.LegacyDB
		resp.ParityGraphs = &parity
		err = withArcLock(root, func() error {
			db, e := openActiveArcDB(root)
			if e != nil {
				return e
			}
			defer db.Close()
			var pending, graphs int
			if e := db.QueryRow("SELECT count(*) FROM arc_outbox o JOIN arc_events e ON o.event_id=e.id WHERE o.status='pending' AND e.aggregate IN ('file','change')").Scan(&pending); e != nil {
				return e
			}
			if e := db.QueryRow("SELECT count(*) FROM graphs").Scan(&graphs); e != nil {
				return e
			}
			policy, pErr := loadPolicyDocument(db)
			if pErr != nil {
				return pErr
			}
			ver := policy.Version
			resp.PendingEvents = &pending
			resp.GraphCount = &graphs
			resp.PolicyStorage = policy.Mode
			resp.PolicyStateVersion = &ver
			return nil
		})
		if err != nil {
			return err
		}
	}
	// 3. 输出具名状态响应JSON
	return emitRawJSON(json.Marshal(resp))
}

// executeDBInitOrMigrate 在仓库锁内完成全新初始化或带门禁等价校验的离线迁移
// 请求体
// root 仓库根目录路径
// action 动作类型（init或migrate）
// confirmOffline 是否已确认停止全部旧写入进程
// 响应体
// err 前置冲突、备份校验或门禁不等价错误
func executeDBInitOrMigrate(root, action string, confirmOffline bool) error {
	// 1. 在仓库排他锁保护下拒绝覆盖已有数据库或清单
	return withArcLock(root, func() error {
		if m, err := readManifest(root); err != nil {
			return err
		} else if m != nil {
			return errors.New("arc database is already active")
		}
		if _, err := os.Lstat(statePath(root)); err == nil {
			return errors.New("unactivated arc.db already exists; inspect/recover explicitly, refusing overwrite")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if _, err := os.Lstat(manifestPath(root)); err == nil {
			return errors.New("arc manifest already exists; refusing overwrite")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		// 2. 初始化私有目录与本地Git忽略配置
		if err := os.MkdirAll(filepath.Join(root, ".arc"), 0o700); err != nil {
			return err
		}
		if err := ensureArcPrivateFiles(root); err != nil {
			return err
		}
		if err := ensureLocalArcExclude(root); err != nil {
			return err
		}
		// 3. 执行离线备份拷贝或调用验证引擎初始化基础表
		m := arcManifest{Version: arcSchemaVersion, Active: true, DB: arcDBName}
		var baseline map[string]gateProof
		if action == "migrate" {
			if !confirmOffline {
				return errors.New("migration requires --confirm-offline after stopping all legacy verification writers")
			}
			source := filepath.Join(root, legacyVerifyDB)
			if err := checkRegular(source); err != nil {
				return fmt.Errorf("legacy verification DB: %w", err)
			}
			resolved, err := filepath.EvalSymlinks(source)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, resolved)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return errors.New("legacy database resolves outside repository")
			}
			if err := noLiveSidecars(source); err != nil {
				return err
			}
			original, err := shaFile(source)
			if err != nil {
				return err
			}
			baseline, err = snapshotGateProofs(root, source)
			if err != nil {
				return err
			}
			dir := filepath.Join(root, ".arc", "backups")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
			st, e := os.Lstat(dir)
			if e != nil {
				return e
			}
			if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
				return errors.New("unsafe symlinked backup directory")
			}
			backup := filepath.Join(dir, "verify-"+original[:16]+".db")
			if _, err := os.Lstat(backup); errors.Is(err, os.ErrNotExist) {
				if err := copyFileExact(source, backup); err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
			if err := checkRegular(backup); err != nil {
				return err
			}
			backupHash, err := shaFile(backup)
			if err != nil || backupHash != original {
				return errors.New("legacy backup checksum mismatch; aborting")
			}
			if err := copyFileExact(source, statePath(root)); err != nil {
				return err
			}
			copied, err := shaFile(statePath(root))
			if err != nil || copied != original {
				return errors.New("database changed while copying; arc.db remains inactive")
			}
			m.LegacyDB, m.LegacySHA256 = legacyVerifyDB, original
			m.MigrationTime = nowRFC3339()
		} else {
			if err := checkRegular(filepath.Join(root, legacyVerifyDB)); err == nil {
				return errors.New("legacy graph database exists; use arc db migrate --confirm-offline instead of init")
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if _, code, err := runLegacyVerify(root, statePath(root), "init"); err != nil || code != 0 {
				return fmt.Errorf("initialize verification schema: error=%v exit=%d", err, code)
			}
		}
		// 4. 创建 arc_* 表结构并执行WAL截断落盘
		db, err := openArcDB(statePath(root))
		if err != nil {
			return err
		}
		err = createArcSchema(db)
		if err == nil {
			var n int
			if scanErr := db.QueryRow("SELECT count(*) FROM graphs").Scan(&n); scanErr != nil {
				err = fmt.Errorf("verification graph tables absent after init/migration: %w", scanErr)
			}
		}
		if err == nil {
			_, err = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		}
		_ = db.Close()
		if err != nil {
			return err
		}
		// 5. 迁移模式下复核旧库未变动且新旧门禁证明完全一致后写入激活清单
		if action == "migrate" {
			if err := noLiveSidecars(filepath.Join(root, legacyVerifyDB)); err != nil {
				return err
			}
			h, err := shaFile(filepath.Join(root, legacyVerifyDB))
			if err != nil || h != m.LegacySHA256 {
				return errors.New("legacy database changed during migration; arc.db NOT activated")
			}
			n, err := verifyParity(root, filepath.Join(root, legacyVerifyDB), statePath(root), baseline)
			if err != nil {
				return err
			}
			m.ParityGraphs = n
		}
		if err := writeManifest(root, m); err != nil {
			return err
		}
		return emitRawJSON(json.Marshal(dbActivateResponse{Database: statePath(root), Active: true, MigratedGraphs: m.ParityGraphs}))
	})
}
