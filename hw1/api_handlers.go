package main

import "net/http"
import "encoding/json"
import "errors"
import "context"

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
	// fmt.Println("    response:" + string(bytes) + "\n")
	w.Write(bytes)
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
	
	

	ctx := context.Background()
	data, err := srv.Profile(ctx, ProfileParams{})
	if err != nil {
		response(w, &ApiError{http.StatusOK, err}, nil)
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
	

	ctx := context.Background()
	data, err := srv.Create(ctx, CreateParams{})
	if err != nil {
		response(w, &ApiError{http.StatusOK, err}, nil)
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
	

	ctx := context.Background()
	data, err := srv.Create(ctx, OtherCreateParams{})
	if err != nil {
		response(w, &ApiError{http.StatusOK, err}, nil)
		return
	}

	response(w, &ApiError{http.StatusOK, errors.New("")}, data)
}

