package main

// ResponseData specifies the "data" structure of the response returned by the function, according to
// [JSONRequest](http://www.json.org/JSONRequest.html) specification.
type ResponseData struct {
	// State species the name of the stage reaches by the function after execution.
	// May help to determine the location of the error.
	Stage string `json:"stage"`
}
