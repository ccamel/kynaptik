package main

type ctxKey string

var (
	ctxKeyValidate             = ctxKey("validate")
	ctxKeyConfig               = ctxKey("config")
	ctxKeySecret               = ctxKey("secret")
	ctxKeyPreConditionProgram  = ctxKey("pre-condition-program")
	ctxKeyPostConditionProgram = ctxKey("post-condition-program")
	ctxKeyData                 = ctxKey("data")
	ctxKeyEnv                  = ctxKey("environment")
	ctxKeyAction               = ctxKey("action")
)
