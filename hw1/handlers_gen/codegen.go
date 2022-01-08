// "Оставь веру всяк сюда входящий" - Данте Алигьери
// Божественная комедия, («Ад», песнь 3, строфа 3)

package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"strings"
	"text/template"
)

type GenParams struct {
	Url    string `json:"url"`
	Auth   bool   `json:"auth"`
	Method string `json:"method"`
}

var (
	respAction = template.Must(template.New("respAction").Parse(`type HTTPResponse struct {
	Error    string      ` + "\x60" + `json:"error"` + "\x60" + `
	Response interface{} ` + "\x60" + `json:"response,omitempty"` + "\x60" + `
}

func response(w http.ResponseWriter, apiErr *ApiError, res interface{}) {
	w.WriteHeader(apiErr.HTTPStatus)
	resp := &HTTPResponse{
		Error:    apiErr.Error(),
		Response: res,
	}
	bytes, _ := json.Marshal(resp)
	w.Write(bytes)
}

func getParamFromPost(postParams string, key string) string {
	for _, p := range strings.Split(postParams, ",") {
		kv := strings.Split(p, "=")
		k := kv[0]
		if k == key {
			if len(kv) > 1 {
				return kv[1]
			} else {
				return k
			}
		}
	}
	return ""
}
`))
)

var (
	serveHttpTmp = template.Must(template.New("serveHttpTmp").Parse(`{{ $apiStructName := .ApiStructName }}
func (srv *{{$apiStructName}}) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	{{range $handler := .Handlers}}
	case "{{.Params.Url}}":
		srv.{{$handler.Name}}HTTPHandler(w, r)
		return
	{{end}}
	default:
		response(w, &ApiError{http.StatusNotFound, errors.New("unknown method")}, nil)
		return
	}
}
{{range $handler := .Handlers}}
func (srv *{{$apiStructName}}) {{$handler.Name}}HTTPHandler(w http.ResponseWriter, r *http.Request) {
	{{if $handler.Params.Method }}
	if r.Method != "{{$handler.Params.Method}}" {
		response(w, &ApiError{http.StatusNotAcceptable, errors.New("bad method")}, nil)
		return
	}
	{{end}}
	{{if eq $handler.Params.Method "POST" }}
	headerValue, ok := r.Header["X-Auth"]
	if !ok {
		response(w, &ApiError{http.StatusForbidden, errors.New("unauthorized")}, nil)
		return
	}
	if len(headerValue) != 1 && headerValue[0] != "100500" {
		response(w, &ApiError{http.StatusForbidden, errors.New("unauthorized")}, nil)
		return
	}
	{{end}}

	body, _ := ioutil.ReadAll(r.Body)
	queryParams := queryParamsToMap(r.URL.Query(), string(body), r.Method)
	
	// Создаем пустые переменные под параметры
	{{range $urlParam := .ParamFields}}
	param{{.Name}}, err, statusCode := validParam{{if eq .Type "string"}}Str{{else}}Int{{end}}("{{.Name}}", {{.Tags}}, queryParams)
	if err != nil {
		response(w, &ApiError{statusCode, err}, nil)
		return
	}
	{{end}}
	// Структура параметров для слоя стора
	urlParams := {{$handler.ParamsStructName}}{
		{{range $urlParam := .ParamFields}}
		{{.Name}}: param{{.Name}},{{end}}
	}

	ctx := context.Background()
	data, err := srv.{{$handler.Name}}(ctx, urlParams)
	if err != nil {
		var statusCode int
		if err.Error() == "user not exist" {
			statusCode = http.StatusNotFound
		} else {
			if strings.Contains(err.Error(), "exist") && !strings.Contains(err.Error(), "not")  {
				statusCode = http.StatusConflict
			} else {
				statusCode = http.StatusInternalServerError
			}
		}
		response(w, &ApiError{statusCode, err}, nil)
		return
	}

	response(w, &ApiError{http.StatusOK, errors.New("")}, data)
}
{{end}}
`))
)

var (
	urlParamsValidator = template.Must(template.New("urlParamsValidator").Parse(`
type Restrictions struct {
	Required bool
	Min *int
	Max *int
	ParamName string
	Enum *struct {
		List []string
		Default string
	}
	Default string
}

func parseRestrictions(restr string) *Restrictions {
	restrictions := &Restrictions{}
	for _, pair := range strings.Split(restr, ",") {
		if pair == "required" {
			restrictions.Required = true
		}

		kv := strings.Split(pair, "=")
		if len(kv) == 2 {
			k := kv[0]
			v := kv[1]

			if k == "min" {
				min, _ := strconv.Atoi(v)
				restrictions.Min = &min
			}

			if k == "max" {
				max, _ := strconv.Atoi(v)
				restrictions.Max = &max
			}

			if k == "paramname" {
				restrictions.ParamName = v
			}

			if k == "enum" {
				values := strings.Split(v, "|")
				restrictions.Enum = &struct{List []string; Default string}{
					List: values,
				}
			}

			if k == "default" {
				if restrictions.Enum != nil {
					restrictions.Enum.Default = v
				} else {
					restrictions.Enum = &struct{List []string; Default string}{
						Default: v,
					}
				}
			}
		}
	}

	return restrictions
}

func queryParamsToMap(getParams map[string][]string, postParams string, method string) map[string]string {
	println("parsing for method:", method)
	values := map[string]string{}

	if method == "POST" {
		for _, kv := range strings.Split(postParams, "&") {
			pair := strings.Split(kv, "=")
			if len(pair) == 2 {
				k := pair[0]
				v := pair[1]
				values[k] = v
			}
		}
	} else {
		for k, arr := range getParams {
			values[k] = arr[0]
		}
	}

	return values
}

func validParamStr(paramName string, restrRaw string, queryParams map[string]string) (string, error, int) {
	restr := parseRestrictions(restrRaw)
	
	var name string
	if restr.ParamName != "" {
		name = restr.ParamName
	} else {
		name = strings.ToLower(paramName)
	}
	value, _ := queryParams[name]
	if restr.Required && value == "" {
		return "", errors.New(name + " must me not empty"), http.StatusBadRequest
	}

	if restr.Max != nil && len(value) > *restr.Max {
		return "", errors.New(name + " len must be <= " + fmt.Sprint(*restr.Max)), http.StatusBadRequest
	}

	if restr.Min != nil && len(value) < *restr.Min {
		return "", errors.New(name + " len must be >= " + fmt.Sprint(*restr.Min)), http.StatusBadRequest
	}

	if restr.Enum != nil {
		if value == "" {
			return restr.Enum.Default, nil, 0
		}

		if !contains(restr.Enum.List, value) {
			return "", errors.New(name + " must be one of [" +  strings.Join(restr.Enum.List, ", ") + "]"), http.StatusBadRequest
		}
	}
	
	return value, nil, 200
}

func validParamInt(paramName string, restrRaw string, queryParams map[string]string) (int, error, int) {
	restr := parseRestrictions(restrRaw)

	var name string
	if restr.ParamName != "" {
		name = restr.ParamName
	} else {
		name = strings.ToLower(paramName)
	}
	value, _ := queryParams[name]
	if restr.Required && value == "" {
		return 0, errors.New(name + " must me not empty"), http.StatusBadRequest
	}

	num, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.New(name + " must be int"), http.StatusBadRequest
	}

	if restr.Max != nil && num > *restr.Max {
		return 0, errors.New(name + " must be <= " + fmt.Sprint(*restr.Max)), http.StatusBadRequest
	}

	if restr.Max != nil && num < *restr.Min {
		return 0, errors.New(name + " must be >= " + fmt.Sprint(*restr.Min)), http.StatusBadRequest
	}

	return num, nil, 200
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

`))
)

type ParamField struct {
	Name string
	Type string
	Tags string
}

type HandlerParams = map[string][]ParamField

type HttpHandlerData struct {
	Name             string
	Params           GenParams
	ParamsStructName string
	ParamFields      []ParamField
}

type serverStructName = string

type HTTPHandlers map[serverStructName][]*HttpHandlerData

func main() {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}
	out, _ := os.Create(os.Args[2])

	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out)
	fmt.Fprintln(out, `import "net/http"`)
	fmt.Fprintln(out, `import "encoding/json"`)
	fmt.Fprintln(out, `import "errors"`)
	fmt.Fprintln(out, `import "strings"`)
	fmt.Fprintln(out, `import "strconv"`)
	fmt.Fprintln(out, `import "context"`)
	fmt.Fprintln(out, `import "io/ioutil"`)
	fmt.Fprintln(out, `import "fmt"`)
	fmt.Fprintln(out)
	respAction.Execute(out, nil)
	urlParamsValidator.Execute(out, nil)

	httpHandlers := HTTPHandlers{}
	handlerParams := HandlerParams{}

	// Ищем объявления структур
	for _, dec := range node.Decls {
		// ast.GenDecl парсит import, constant, type or variable declaration
		// Нам нужен type
		genDecs, ok := dec.(*ast.GenDecl)
		// Если спарсил не функцию
		if ok {
			for _, spec := range genDecs.Specs {
				if currType, ok := spec.(*ast.TypeSpec); ok {
					// Если спарсили type
					typeName := currType.Name.Name
					if currStruct, ok := currType.Type.(*ast.StructType); ok && strings.Contains(typeName, "Params") {
						// Если спарсили struct
						handlerParams[typeName] = []ParamField{}

						// Идем по массиву полей структуры и вытаскиваем тип поля и его структурные теги
						for _, field := range currStruct.Fields.List {
							fieldName := field.Names[0].Name
							fieldType := parseFieldType(field)
							fieldTags := clearStructTags(field.Tag.Value)
							handlerParams[typeName] = append(
								handlerParams[typeName],
								ParamField{fieldName, fieldType, fieldTags},
							)
						}
					}
				}
			}
		}

		funcDecl, ok := dec.(*ast.FuncDecl)
		// Handle only recievers with docs
		if ok && (funcDecl.Doc != nil) && (funcDecl.Recv != nil) {
			var recvTypeName string         // MyApi
			var recvParamsStructName string // ProfileParams
			for _, recv := range funcDecl.Recv.List {
				recvTypeName = parseRecieverType(recv)
				break
			}

			recvParamsStructName = parseRecieverParamsStructName(funcDecl)

			var genParams *GenParams
			for _, doc := range funcDecl.Doc.List {
				if strings.Contains(doc.Text, "apigen:api") {
					genParams = parseDocs(doc.Text)
					httpHandlers[recvTypeName] = append(httpHandlers[recvTypeName], &HttpHandlerData{
						Name:             funcDecl.Name.Name,
						Params:           *genParams,
						ParamsStructName: recvParamsStructName,
					})
					break
				}
			}
		}
	}

	for _, v := range httpHandlers {
		for _, handler := range v {
			queryParams, ok := handlerParams[handler.ParamsStructName]
			if ok {
				handler.ParamFields = queryParams
			}
		}
	}

	writeHTTPHandlers(out, httpHandlers)
}

func parseDocs(rawStr string) *GenParams {
	firstStructPos := strings.Index(rawStr, "{")
	lastStructPos := strings.Index(rawStr, "}")
	stringJson := rawStr[firstStructPos : lastStructPos+1]

	params := &GenParams{}
	json.Unmarshal([]byte(stringJson), params)

	return params
}

func parseRecieverType(recv *ast.Field) (typeName string) {
	switch xv := recv.Type.(type) {
	case *ast.StarExpr:
		if si, ok := xv.X.(*ast.Ident); ok {
			typeName = si.Name
		}
	case *ast.Ident:
		typeName = xv.Name
	}

	return
}

func parseRecieverParamsStructName(funcDecl *ast.FuncDecl) (typeName string) {
	for i, paramType := range funcDecl.Type.Params.List {
		if i == 1 {
			switch xv := paramType.Type.(type) {
			case *ast.StarExpr:
				if si, ok := xv.X.(*ast.Ident); ok {
					typeName = si.Name
				}
			case *ast.Ident:
				typeName = xv.Name
			}
		}
	}
	return
}

func parseFieldType(field *ast.Field) (typeName string) {
	switch xv := field.Type.(type) {
	case *ast.StarExpr:
		if si, ok := xv.X.(*ast.Ident); ok {
			typeName = si.Name
		}
	case *ast.Ident:
		typeName = xv.Name
	}
	return
}

func writeHTTPHandlers(out *os.File, data HTTPHandlers) {
	for k, v := range data {
		templateData := &struct {
			ApiStructName string
			Handlers      []*HttpHandlerData
		}{
			ApiStructName: k,
			Handlers:      v,
		}
		serveHttpTmp.Execute(out, templateData)
	}
}

func clearStructTags(tags string) string {
	return tags[1+len("apivalidator:") : len(tags)-1]
}

// var urlParam string
// 	urlQuery := r.URL.Query()
// 	{{range $urlParam := .ParamFields}}
// 	if r.Method == http.MethodPost {
// 		body, _ := ioutil.ReadAll(r.Body)
// 		urlParam = string(body)
// 	}
// 	if r.Method == http.MethodGet {
// 		urlParam = urlQuery.Get(strings.ToLower("{{.Name}}"))
// 	}

// param{{.Name}}, err, statusCode := validateParam(
// 	strings.ToLower("{{.Name}}"),
// 	urlParam,
// 	"{{.Type}}",
// 	{{.Tags}},
// )
// if err != nil {
// 	response(w, &ApiError{statusCode, err}, nil)
// 	return
// }
// param{{.Name}}Parsed, ok := param{{.Name}}.({{.Type}})
// if !ok {

// }
// {{end}}
