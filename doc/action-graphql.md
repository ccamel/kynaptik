# graphql(s)://

> Provides [GraphQL][graphql] actions for calling external [GraphQL][graphql] APIs.

## Description

The query is sent following the [HTTP protocol](https://graphql.org/learn/serving-over-http/):

-   using HTTP method `POST`
-   using the `application/json` content type
-   including a JSON-encoded body of the following form:

```json
{
  "query": "...",
  "operationName": "...",
  "variables": { "myVariable": "someValue", ... }
}
```

## URI

`graphql[s]://hostname[:port][/graphQLEndpoint]`

## Configuration

The `action` yaml element supports the following elements: 

| Field | Type | Req. | Default value | Description |
|--------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|---------------|----------------------------------------------------------------------------|
| `uri` | URI according to [rfc3986](https://www.ietf.org/rfc/rfc3986.txt). | ✓ |  | The URL to invoke.<br/>`graphql`: relative to an `http` request<br/>`graphqls` : relative to an `https` request |
| `query` | `graphQL` query (textual). | ✓ | | [GraphQL query](https://graphql.org/learn/queries/) to send. |
| `variables` | name/value `map`. |  |  | [GraphQL variables](https://graphql.org/learn/queries/#variables) to use. |
| `operationName` | `string` |  |  | The name of the operation - only required if multiple operations are present in the query. |

## Evaluation environment

The environment variable `response` is the [HTTP response](https://golang.org/pkg/net/http/#Response).

## Example

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: default
  name: kynaptik-graphql-configmap
data:
  function-spec: |
    timeout: 10000
    preCondition: |
      data.name != ""

    action: |
      uri: 'graphqls://graphql-pokemon.now.sh/?'                        
      query: |
        query ViewPokemon($name: String) {
          pokemon(name: $name) {
            id
            number
            name
            attacks {
              special {
                name
                type
                damage
              }
            }
          }
        }
      variables:        
        name: '{{ .data.name }}'        
    postCondition: |
      response.StatusCode == 200
```

[graphql]: https://graphql.org/
