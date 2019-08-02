Кynaptiꓘ
========

![under-construction](https://img.shields.io/badge/%F0%9F%9A%A7-under%20construction-important)
[![build-status](https://travis-ci.org/ccamel/kynaptik.svg?branch=master)](https://travis-ci.org/ccamel/kynaptik)
[![coverage-status](https://coveralls.io/repos/github/ccamel/kynaptik/badge.svg?branch=feat/add-coverage-ci)](https://coveralls.io/github/ccamel/kynaptik?branch=feat/add-coverage-ci)
[![maintainability](https://api.codeclimate.com/v1/badges/bb38e3df1b0591b4d1ef/maintainability)](https://codeclimate.com/github/ccamel/kynaptik/maintainability)
[![stackshare](http://img.shields.io/badge/tech-stack-0690fa.svg?style=flat)](https://stackshare.io/ccamel/kynaptik)

> Serverless Function on [Kubernetes] (through [Fission](https://fission.io/)) providing a generic and configurable mean to trigger _actions_ from incoming _events_.

## Purpose

`Kynaptiꓘ` is a function which specifies how a stimulus (i.e. incoming request, message) elicits a response (i.e. invocation of endpoint).

More broadly, it provides a platform with a versatile and generic Web Hook serving multiple purposes.

## Principles

`Kynaptiꓘ` is a function that deploys on [Fission] [FaaS](https://en.wikipedia.org/wiki/Function_as_a_service), an amazing framework for serverless functions on [Kubernetes].

The following diagram depicts how the main components interact:

![overview](doc/kynaptik-overview.png)

## Features

`Kynaptiꓘ` is a simple, lean function with a deliberately limited set of features. Development is driven by the [KISS](https://en.wikipedia.org/wiki/KISS_principle) principle:
"do one thing well".

- Conditional actions: an action is executed only in case a message matches the defined condition. That condition is specified by an [expression](https://github.com/antonmedv/expr) evaluated
at running time against an environment containing the incoming message.
- Extensible configuration of actions with templating: URL, HTTP method, headers and body.

__Out of scope:__

- No complex conditions, e.g. based on a state based on time ([CEP](https://en.wikipedia.org/wiki/Complex_event_processing))
- No content enrichment: no way to access an external data source in order to augment a message with missing information.
The incoming messages are expected to be qualified enough for the processing.
- Fire and forget behavior: the action (e.g. HTTP post) is done once, the result is not used (a log is emitted though)
- No recovery policy: no retry if the action fails

## Configuration

### configmap

`Kynaptiꓘ` is configured by a k8s `ConfigMap` which defines the configuration for the function.

The `ConfigMap` _shall_ declares a key in `data` which contain the `yaml` configuration of the function.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: default
  name: kynaptik-http-configmap
data:
  function-spec: |
    condition: |
      data.foo == "bar"

    action:
      uri: 'https://foo-bar'
      method: GET
      headers:
        Content-Type: application/json
        X-Userid: |
          {{if eq .data.user.firstname "john"}}Rmlyc3Qgb3B0aW9u={{else}}U2Vjb25kIG9wdGlvbg=={{end}}
      body: |
        {
          "message": "Hello from {{.data.user.firstname}} {{.data.user.lastname}}"
        }
```

The yaml configuration has the following structure:

- `condition`: mandatory, specifies the condition (textual) to be satisfied for the function to be triggered. The condition is an expression 
(text) compliant with the syntax of [antonmedv/expr](https://github.com/antonmedv/expr) engine.
- `action`: specifies the action to perform.
  - `uri`: mandatory, the URI of the endpoint to invoke. Shall resolve to a URI according to [rfc3986](https://www.ietf.org/rfc/rfc3986.txt).
  Protocol shall be either `http` or `https`.
  - `method`: mandatory, the HTTP method: `GET`, `OPTIONS`, `GET`, `HEAD`, `POST`, `PUT`, `DELETE`, `TRACE`, `CONNECT` (according to [rfc2616](https://www.ietf.org/rfc/rfc2616.txt)).
  - `headers`: the HTTP headers as a map key/value.
  - `body`: the content of the body (texttual).

The action specification is _templated_ using the [go template engine](https://golang.org/pkg/text/template/). See section below to have
details about the evaluation environment.

## Evaluation environment

Both the _condition_ expression and the _action_ template are processed against an environment.

The environment is following:

- `data`: the incoming message (_body_ only), serialized into a map structure, with preservation of primary types (numbers, strings).
- `config`: the current configuration.


[Kubernetes]: https://kubernetes.io/
[Fission]: https://fission.io/
[Kafka]: https://kafka.apache.org/
