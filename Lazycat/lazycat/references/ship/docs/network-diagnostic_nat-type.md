# Source: https://developer.lazycat.cloud/network-diagnostic/nat-type.md

# NAT Type

假如您配置了代理：

* 请检查代理分流规则，确保开启了"绕过中国大陆"选项。并且配置"6.6.6.6"和"2000::6666"绕过代理。

Clash

```
rules:
  - GEOIP,CN,DIRECT
tun:
  route-exclude-address:
    - 6.6.6.6/32
    - 2000::6666/128
```

如果您没有配置代理，您的运营商可能配置了NAT4。

您可以联系运营商要求更改成NAT3，或者考虑更换成别的运营商。