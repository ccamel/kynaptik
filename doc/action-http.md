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
| `options:`<br/>&nbsp;&nbsp;`transport:`<br/>&nbsp;&nbsp;&nbsp;&nbsp;`followRedirect` | `boolean`. |  | `true` | Tell to follow redirects. |
| `options:`<br/>&nbsp;&nbsp;`transport:`<br/>&nbsp;&nbsp;&nbsp;&nbsp;`maxRedirects` | `positive integer`. |  | `50` | Specifies the maximum number of redirects to follow. |
| `options:`<br/>&nbsp;&nbsp;`tls:`<br/>&nbsp;&nbsp;&nbsp;&nbsp;`caCertData` | `string`. |  |  | Root certificate authority that the client use when verifying server certificates. |
| `options:`<br/>&nbsp;&nbsp;`tls:`<br/>&nbsp;&nbsp;&nbsp;&nbsp;`clientCertData` | `string`. |  |  | PEM encoded data of the public key. |
| `options:`<br/>&nbsp;&nbsp;`tls:`<br/>&nbsp;&nbsp;&nbsp;&nbsp;`clientKeyData` | `string`. |  |  | PEM encoded data of the private key. |
| `options:`<br/>&nbsp;&nbsp;`tls:`<br/>&nbsp;&nbsp;&nbsp;&nbsp;`insecureSkipVerify` | `boolean`. |  | `false` | Controls whether the client verifies the server's certificate chain and host name. :warning: if `true`, TLS is susceptible to man-in-the-middle attacks. |
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
        transport:
          followRedirect: true
          maxRedirects: 10
```
