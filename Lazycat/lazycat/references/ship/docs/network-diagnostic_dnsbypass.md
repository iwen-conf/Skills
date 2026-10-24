# Source: https://developer.lazycat.cloud/network-diagnostic/dnsbypass.md

# DNS

* 在代理配置中，添加规则绕过"\*.heiyu.space"。

* Configure your proxy to bypass "\*.heiyu.space".

Clash:

```
dns:
  fake-ip-filter:
    - "*.heiyu.space"
```