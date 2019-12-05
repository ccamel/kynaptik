# http(s)://

> Provides HTTP actions for calling external HTTP(S) resources.

## URI

`http[s]://hostname[:port][/resourceUri][?options]`

## Configuration

The `action` yaml element supports the following additional elements: 

-   `method`: mandatory, the HTTP method: `GET`, `OPTIONS`, `GET`, `HEAD`, `POST`, `PUT`, `DELETE`, `TRACE`, `CONNECT` (according to [rfc2616](https://www.ietf.org/rfc/rfc2616.txt)).
-   `headers`: the HTTP headers as a map key/value.
-   `body`: the content of the body (textual).

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
```
