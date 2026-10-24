# Source: https://developer.lazycat.cloud/advanced-error-template.md

# 错误页面

当应用出错时， 可以通过在 `lzc-manifest.yml` 文件中的 `application` 字段下加一个 `handlers` 子字段即可:

```yml
application:
  handlers:
    error_page_templates:
      502: /lzcapp/pkg/content/errors/502.html.tpl
      404: /lzcapp/pkg/content/errors/404.html.tpl
```

上面配置的意思是， 当微服系统检测到 HTTP 错误时， 会根据配置的错误代码自动去找应用提供的错误页面模板。

默认的模板是:

```html
<html>
  <body>
    <h1>
      您的应用发生错误啦！
    </h1>
    <p>
      失败原因: {{ .ErrorDetail}}
    </p >
    <p>
      请稍后再试吧
    </p >
  </body>
</html>
```

* `ErrorDetail`： 是微服内置的错误代码， 当应用发生错误时， 微服系统会用容器失败日志内容替换 `ErrorDetail` ， 帮助开发者排查错误