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

Where the supported protocols are :   

-   `graphql`: relative to an `http` request
-   `graphqls` : relative to an `https` request

## Configuration

The `action` yaml element supports the following additional elements: 

-   `query`: mandatory, [GraphQL query](https://graphql.org/learn/queries/) to send.
-   `variables`: optional, the [GraphQL variables](https://graphql.org/learn/queries/#variables) to use.
-   `operationName`: optional, the name of the operation - only required if multiple operations are present in the query.

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
    preCondition: |
      data.name != ""

    action: |
      uri: 'graphqls://graphql-pokemon.now.sh/?'      
      timeout: 10000            
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
