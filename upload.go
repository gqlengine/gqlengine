// Copyright 2020 凯斐德科技（杭州）有限公司 (Karfield Technology, ltd.)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package gqlengine

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"reflect"
	"strconv"
	"strings"

	"github.com/karfield/graphql/language/ast"

	"github.com/karfield/graphql"
)

var UploadScalar = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "Upload",
	Description: "The `Upload` scalar type represents a file upload.",
	Serialize: func(value interface{}) interface{} {
		panic("Upload serialization unsupported.")
		return nil
	},
	ParseValue: func(value interface{}) interface{} {
		return value
	},
	ParseLiteral: func(valueAST ast.Value) interface{} {
		panic("Upload literal unsupported.")
	},
})

type Upload struct {
	*multipart.FileHeader
}

var (
	_uploadType = reflect.ValueOf(Upload{}).Type()
)

func asUploadScalar(field *reflect.StructField) (graphql.Type, *unwrappedInfo, error) {
	unwrapped, err := unwrap(field.Type)
	if err != nil {
		return nil, nil, err
	}
	if unwrapped.ptrType == _uploadType || unwrapped.baseType == _uploadType || unwrapped.implType == _uploadType {
		unwrapped.baseType = _uploadType
		unwrapped.implType = _uploadType
		return wrapType(field, UploadScalar, unwrapped.array), &unwrapped, nil
	} else {
		return nil, nil, nil
	}
}

func fixVariablesWithJsonPath(variables map[string]interface{}, path string, value interface{}) error {
	dotIdx := strings.Index(path, ".")
	if dotIdx < 0 {
		variables[path] = value
	} else {
		pre := path[0:dotIdx]
		if sub, ok := variables[pre]; ok {
			if slice, ok := sub.([]interface{}); ok {
				idxStr := path[dotIdx+1:]
				arrDotIdx := strings.Index(idxStr, ".")
				if arrDotIdx < 0 {
					idx, err := strconv.Atoi(idxStr)
					if err != nil {
						return errors.New("unmatched json path")
					}
					if idx >= len(slice) {
						return errors.New("unmatched json path")
					}
					slice[idx] = value
					// no more
					return nil
				} else {
					subPath := idxStr[arrDotIdx+1:]
					idxStr = idxStr[0:arrDotIdx]
					idx, err := strconv.Atoi(idxStr)
					if err != nil {
						return errors.New("unmatched json path")
					}
					if idx >= len(slice) {
						return errors.New("unmatched json path")
					}
					if sub, ok := slice[idx].(map[string]interface{}); ok {
						return fixVariablesWithJsonPath(sub, subPath, value)
					} else {
						return errors.New("unmatched json path")
					}
				}
			} else if subVars, ok := sub.(map[string]interface{}); ok {
				return fixVariablesWithJsonPath(subVars, path[dotIdx+1:], value)
			} else {
				return errors.New("unmatched json path")
			}
		} else {
			return errors.New("unmatched json path")
		}
	}
	return nil
}

func getFromMultipart(form *multipart.Form) []*RequestOptions {
	var optsList []*RequestOptions
	if operations, ok := form.Value["operations"]; !ok || len(operations) == 0 {
		return nil
	} else {
		batching := operations[0][0] == '['
		if batching {
			if err := json.Unmarshal([]byte(operations[0]), &optsList); err != nil {
				return nil
			}
		} else {
			opts := RequestOptions{}
			if err := json.Unmarshal([]byte(operations[0]), &opts); err != nil {
				return nil
			}
			optsList = []*RequestOptions{&opts}
		}
	}

	variables := map[string][]string{}
	if _map, ok := form.Value["map"]; ok {
		if err := json.Unmarshal([]byte(_map[0]), &variables); err != nil {
			return nil
		}
	} else {
		return nil
	}

	fixVariables := func(opts *RequestOptions, prefix string) error {
		for k, v := range variables {
			path := v[0]
			if strings.HasPrefix(path, prefix) {
				path = strings.TrimPrefix(path, prefix)
				if err := fixVariablesWithJsonPath(opts.Variables, path, Upload{
					FileHeader: form.File[k][0],
				}); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if len(optsList) > 1 {
		for i, opts := range optsList {
			prefix := fmt.Sprintf("%d.variables.", i)
			if err := fixVariables(opts, prefix); err != nil {
				return nil
			}
		}
	} else {
		if err := fixVariables(optsList[0], "variables."); err != nil {
			return nil
		}
	}
	return optsList
}
