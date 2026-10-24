# Source: https://developer.lazycat.cloud/advanced-db.md

# 数据库服务

## 启动数据库服务

当您需要启动类似 MySQL 这样的数据库服务时， 只需要在 `lzc-manifest.yml` 文件中加入下面内容即可， 微服会自动启动一个数据库容器， 对外提供数据库读写服务， 默认监听 `3306` 端口。

```yml
services:
  mysql:
    image: registry.lazycat.cloud/mysql
    binds:
      - /lzcapp/var/mysql:/var/lib/mysql
    environment:
      - MYSQL_ROOT_PASSWORD=LAZYCAT
      - MYSQL_DATABASE=LAZYCAT
      - MYSQL_USER=LAZYCAT
      - MYSQL_PASSWORD=LAZYCAT
```

配置详细解释：

* `mysql`: 数据库 Docker 服务的名字
* `image`: 从懒猫微服下载 MySQL Docker 的地址
* `binds`: 当 MySQL 写入数据到 `/var/lib/mysql` 时， 微服系统会自动绑定到应用容器的 `/lzcapp/var/mysql` 路径
* `environment`: 定义 MySQL 启动所需的环境变量

## 连接数据库服务

一旦数据库服务启动后， 访问也很简单， 只需要通过 `mysql.package.lzcapp:3306` 的方式就可以访问。

比如， 应用的 `package` 为 `cloud.lazycat.app.todolistpy`， 在代码中只需要访问 `mysql.cloud.lazycat.app.todolistpy.lzcapp:3306` 就可以自由的读写 MySQL 啦。

## 其他数据库服务

当您需要启动类似PostgreSQL这样的数据库服务时， 只需要在 `lzc-manifest.yml` 文件中加入下面内容即可， 微服会自动启动一个数据库容器， 对外提供数据库读写服务， 默认监听 `5432` 端口。

```yml
services:
  postgresql:
    image: registry.lazycat.cloud/postgres:18.1
    environment:
      - POSTGRES_USER=lazycat
      - POSTGRES_PASSWORD=lazycat
      - POSTGRES_DB=lazycat
    binds:
      - /lzcapp/var/pgdata:/var/lib/postgresql
```

当您需要启动类似Redis这样的数据库服务时， 只需要在 `lzc-manifest.yml` 文件中加入下面内容即可， 微服会自动启动一个数据库容器， 对外提供数据库读写服务， 默认监听 `6379` 端口。

```yml
services:
  redis:
    image: registry.lazycat.cloud/redis:8.0
    command: redis-server --appendonly yes
```