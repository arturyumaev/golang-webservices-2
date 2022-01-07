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
	fmt.Println("    response:" + string(bytes) + "\n")
	w.Write(bytes)
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

	ctx := context.Background()

	data, err := srv.{{$handler.Name}}(ctx, {{$handler.ParamsStructName}}{})
	if err != nil {
		response(w, &ApiError{http.StatusOK, err}, nil)
		return
	}
	response(w, &ApiError{http.StatusOK, errors.New("")}, data)
}
{{end}}
`))
)

type HttpHandlerData struct {
	Name             string
	Params           GenParams
	ParamsStructName string
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
	fmt.Fprintln(out, `import "context"`)
	fmt.Fprintln(out, `import "fmt"`)
	fmt.Fprintln(out)
	respAction.Execute(out, nil)

	httpHandlers := HTTPHandlers{}

	for _, dec := range node.Decls {
		_, ok := dec.(*ast.GenDecl)
		if ok {
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
