// GENERATED BY THE COMMAND ABOVE; DO NOT EDIT
// This file was generated by swaggo/swag

package docs

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/alecthomas/template"
	"github.com/swaggo/swag"
)

var doc = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{.Description}}",
        "title": "{{.Title}}",
        "termsOfService": "http://swagger.io/terms/",
        "contact": {
            "name": "API Support",
            "url": "http://www.swagger.io/support",
            "email": "support@swagger.io"
        },
        "license": {
            "name": "Apache 2.0",
            "url": "http://www.apache.org/licenses/LICENSE-2.0.html"
        },
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/case/{id}": {
            "get": {
                "produces": [
                    "application/json"
                ],
                "summary": "Get case by case id",
                "parameters": [
                    {
                        "type": "integer",
                        "description": "case ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "allOf": [
                                {
                                    "$ref": "#/definitions/api.ReturnedData"
                                },
                                {
                                    "type": "object",
                                    "properties": {
                                        "data": {
                                            "$ref": "#/definitions/cases.Case"
                                        }
                                    }
                                }
                            ]
                        }
                    }
                }
            }
        },
        "/ping": {
            "get": {
                "description": "respond to a ping request from client",
                "produces": [
                    "application/json"
                ],
                "summary": "Test server up statue",
                "responses": {
                    "200": {
                        "description": "Good, server is up",
                        "schema": {
                            "$ref": "#/definitions/api.ReturnedData"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "api.ReturnedData": {
            "type": "object",
            "properties": {
                "data": {
                    "description": "Anything you want to pass to the frontend, but make it simple and necessary\nIf there's nothing to return, this field will be omitted",
                    "type": "object"
                },
                "status": {
                    "description": "A simple string indicating the status.\nIs it ok, or some error occurs? If so, what is the error?\nIt should be \"ok\" is everything goes fine",
                    "type": "string"
                }
            }
        },
        "cases.Case": {
            "type": "object",
            "properties": {
                "complaint": {
                    "type": "string"
                },
                "diagnosis": {
                    "type": "string"
                },
                "id": {
                    "description": "Every object should have ID",
                    "type": "integer"
                },
                "patientID": {
                    "description": "A has many relationship should be on this",
                    "type": "integer"
                },
                "prescriptions": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/cases.Prescription"
                    }
                }
            }
        },
        "cases.Prescription": {
            "type": "object",
            "properties": {
                "caseID": {
                    "type": "integer"
                },
                "details": {
                    "type": "string"
                },
                "id": {
                    "type": "integer"
                }
            }
        }
    }
}`

type swaggerInfo struct {
	Version     string
	Host        string
	BasePath    string
	Schemes     []string
	Title       string
	Description string
}

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = swaggerInfo{
	Version:     "1.0",
	Host:        "localhost:12448",
	BasePath:    "/api",
	Schemes:     []string{},
	Title:       "Swagger Example API",
	Description: "This is a sample server Petstore server.",
}

type s struct{}

func (s *s) ReadDoc() string {
	sInfo := SwaggerInfo
	sInfo.Description = strings.Replace(sInfo.Description, "\n", "\\n", -1)

	t, err := template.New("swagger_info").Funcs(template.FuncMap{
		"marshal": func(v interface{}) string {
			a, _ := json.Marshal(v)
			return string(a)
		},
	}).Parse(doc)
	if err != nil {
		return doc
	}

	var tpl bytes.Buffer
	if err := t.Execute(&tpl, sInfo); err != nil {
		return doc
	}

	return tpl.String()
}

func init() {
	swag.Register(swag.Name, &s{})
}
