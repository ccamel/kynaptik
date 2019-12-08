# http(s)://

> Provides HTTP actions for calling external HTTP(S) resources.

## URI

`http[s]://hostname[:port][/resourceUri][?options]`

## Configuration

The `action` yaml element supports the following elements: 

| Field | Type | Req. | Default value | Description |
|--------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|---------------|----------------------------------------------------------------------------|
| `uri` | URI according to [rfc3986](https://www.ietf.org/rfc/rfc3986.txt). | ✓ |  | The URL to invoke, either `http` or `https`. |
| `method` | A `string` among the following values: `GET`, `OPTIONS`, `GET`, `HEAD`, `POST`, `PUT`, `DELETE`, `TRACE`, `CONNECT` (according to [rfc2616](https://www.ietf.org/rfc/rfc2616.txt)). | ✓ |  | The HTTP method. |
| `headers` | key/value `map`. |  |  | The HTTP headers (key/value set). |
| `body` | `text` |  |  | The content of the body. |
| `options:`<br/>&nbsp;&nbsp;&nbsp;`followRedirect` | `boolean`. |  | `true` | Tell to follow redirects. |
| `options:`<br/>&nbsp;&nbsp;&nbsp;`maxRedirects` | `positive integer`. |  | `50` | Specifies the maximum number of redirects to follow. |
| `timeout` | `positive integer`. |  |  | Specifies the timeout for waiting for data (in ms). No timeout by default. |

## Evaluation environment

The environment variable `response` is the [HTTP response](https://golang.org/pkg/net/http/#Response). 

## Example

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: default
  name: kynaptik-http-configmap
data:
  function-spec.yml: |
    preCondition: |
      data.message != ""

    action: |
      uri: 'https://webhook.site/{{ .data.key }}'
      method: POST
      headers:
        Content-Type: application/json

      body: |
        {
          message: {{ .data.message }}
        }
      options:
        followRedirect: true
        maxRedirects: 10
```
