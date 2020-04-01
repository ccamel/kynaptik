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

## Evaluation environment

The environment variable `response` is the [HTTP response](https://golang.org/pkg/net/http/#Response). 

## Installation

### Package deployment

Before creating a function, you’ll need an environment for the Go language. Read [environments](https://docs.fission.io/docs/usage/environments/) if you haven’t already.

Create a new package from a released archive:

```sh
> fission pkg create --name kynaptik-http --env go --source https://github.com/ccamel/kynaptik/releases/download/v1.0.0/kynaptik-http.zip
```

### Configuration

The behavior of the function is specified by a k8s `ConfigMap` which shall contain the key `function-spec.yml` under which goes all the configuration.

For instance, the following `ConfigMap` configures a function which perfoms a call to [Webhook.site](https://webhook.site/), you can use to test and debug Webhooks and HTTP requests.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: default
  name: webhook
data:
  function-spec.yml: |
    timeout: 10000
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

### Function creation

```sh
> fission fn create --name webhook --env go --pkgname kynaptik-http --entrypoint EntryPoint --configmap webhook
```

### Function test

Go to the URL https://webhook.site and get the unique session id provided, for instance:

```
https://webhook.site/01f84643-93ba-4407-9aa9-ffffffffffff
```

Now you can test the function:

```sh
> fission fn test --name webhook --method=POST --body='{ "key": "01f84643-93ba-4407-9aa9-ffffffffffff", "message":"hello world!" }' --header="Content-Type: application/json"

{"data":{"stage":"match-post-condition"},"message":"HTTP call succeeded","status":"success"}
```

You should view the details of the request on webhook.site page.
