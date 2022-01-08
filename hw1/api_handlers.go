package main

import "net/http"
import "encoding/json"
import "errors"
import "strings"
import "strconv"
import "context"
import "io/ioutil"
import "fmt"

type HTTPResponse struct {
	Error    string      `json:"error"`
	Response interface{} `json:"response,omitempty"`
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


func (srv *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	
	case "/user/profile":
		srv.ProfileHTTPHandler(w, r)
		return
	
	case "/user/create":
		srv.CreateHTTPHandler(w, r)
		return
	
	default:
		response(w, &ApiError{http.StatusNotFound, errors.New("unknown method")}, nil)
		return
	}
}

func (srv *MyApi) ProfileHTTPHandler(w http.ResponseWriter, r *http.Request) {
	
	

	body, _ := ioutil.ReadAll(r.Body)
	queryParams := queryParamsToMap(r.URL.Query(), string(body), r.Method)
	
	// Создаем пустые переменные под параметры
	
	paramLogin, err, statusCode := validParamStr("Login", "required", queryParams)
	if err != nil {
		response(w, &ApiError{statusCode, err}, nil)
		return
	}
	
	// Структура параметров для слоя стора
	urlParams := ProfileParams{
		
		Login: paramLogin,
	}

	ctx := context.Background()
	data, err := srv.Profile(ctx, urlParams)
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

func (srv *MyApi) CreateHTTPHandler(w http.ResponseWriter, r *http.Request) {
	
	if r.Method != "POST" {
		response(w, &ApiError{http.StatusNotAcceptable, errors.New("bad method")}, nil)
		return
	}
	
	
	headerValue, ok := r.Header["X-Auth"]
	if !ok {
		response(w, &ApiError{http.StatusForbidden, errors.New("unauthorized")}, nil)
		return
	}
	if len(headerValue) != 1 && headerValue[0] != "100500" {
		response(w, &ApiError{http.StatusForbidden, errors.New("unauthorized")}, nil)
		return
	}
	

	body, _ := ioutil.ReadAll(r.Body)
	queryParams := queryParamsToMap(r.URL.Query(), string(body), r.Method)
	
	// Создаем пустые переменные под параметры
	
	paramLogin, err, statusCode := validParamStr("Login", "required,min=10", queryParams)
	if err != nil {
		response(w, &ApiError{statusCode, err}, nil)
		return
	}
	
	paramName, err, statusCode := validParamStr("Name", "paramname=full_name", queryParams)
	if err != nil {
		response(w, &ApiError{statusCode, err}, nil)
		return
	}
	
	paramStatus, err, statusCode := validParamStr("Status", "enum=user|moderator|admin,default=user", queryParams)
	if err != nil {
		response(w, &ApiError{statusCode, err}, nil)
		return
	}
	
	paramAge, err, statusCode := validParamInt("Age", "min=0,max=128", queryParams)
	if err != nil {
		response(w, &ApiError{statusCode, err}, nil)
		return
	}
	
	// Структура параметров для слоя стора
	urlParams := CreateParams{
		
		Login: paramLogin,
		Name: paramName,
		Status: paramStatus,
		Age: paramAge,
	}

	ctx := context.Background()
	data, err := srv.Create(ctx, urlParams)
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


func (srv *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	
	case "/user/create":
		srv.CreateHTTPHandler(w, r)
		return
	
	default:
		response(w, &ApiError{http.StatusNotFound, errors.New("unknown method")}, nil)
		return
	}
}

func (srv *OtherApi) CreateHTTPHandler(w http.ResponseWriter, r *http.Request) {
	
	if r.Method != "POST" {
		response(w, &ApiError{http.StatusNotAcceptable, errors.New("bad method")}, nil)
		return
	}
	
	
	headerValue, ok := r.Header["X-Auth"]
	if !ok {
		response(w, &ApiError{http.StatusForbidden, errors.New("unauthorized")}, nil)
		return
	}
	if len(headerValue) != 1 && headerValue[0] != "100500" {
		response(w, &ApiError{http.StatusForbidden, errors.New("unauthorized")}, nil)
		return
	}
	

	body, _ := ioutil.ReadAll(r.Body)
	queryParams := queryParamsToMap(r.URL.Query(), string(body), r.Method)
	
	// Создаем пустые переменные под параметры
	
	paramUsername, err, statusCode := validParamStr("Username", "required,min=3", queryParams)
	if err != nil {
		response(w, &ApiError{statusCode, err}, nil)
		return
	}
	
	paramName, err, statusCode := validParamStr("Name", "paramname=account_name", queryParams)
	if err != nil {
		response(w, &ApiError{statusCode, err}, nil)
		return
	}
	
	paramClass, err, statusCode := validParamStr("Class", "enum=warrior|sorcerer|rouge,default=warrior", queryParams)
	if err != nil {
		response(w, &ApiError{statusCode, err}, nil)
		return
	}
	
	paramLevel, err, statusCode := validParamInt("Level", "min=1,max=50", queryParams)
	if err != nil {
		response(w, &ApiError{statusCode, err}, nil)
		return
	}
	
	// Структура параметров для слоя стора
	urlParams := OtherCreateParams{
		
		Username: paramUsername,
		Name: paramName,
		Class: paramClass,
		Level: paramLevel,
	}

	ctx := context.Background()
	data, err := srv.Create(ctx, urlParams)
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

