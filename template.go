package main

import (
	"bytes"
	"strings"
	"text/template"
)

var httpTemplate = `
{{$svrType := .ServiceType}}
{{$svrName := .ServiceName}}
type {{.ServiceType}}HTTPServer interface {
	Err(*gin.Context, error)
	Data(*gin.Context, interface{})
{{- range .MethodSets}}
	{{.Name}}(*gin.Context, *{{.Request}}) (*{{.Reply}}, error)
{{- end}}
}

type Unimplemented{{.ServiceType}}HTTPServer struct {
}

type Response struct {
	Code int32 ` + "`json:\"code\"`" + `
	Message string ` + "`json:\"message\"`" + `
	Data interface{} ` + "`json:\"data,omitempty\"`" + `
}

func (Unimplemented{{.ServiceType}}HTTPServer) Err(c *gin.Context, err error) {
	res := &Response{}
	if errx, ok := err.(errorx.Errorx); ok {
		res.Code = errx.Code()
	} else {
		res.Code = -1
	}
	res.Message = err.Error()
	c.JSON(200, res)
}

func (Unimplemented{{.ServiceType}}HTTPServer) Data(c *gin.Context, data interface{}) {
	res := &Response{
		Code: 200,
		Message: "ok",
		Data: data,
	}
	c.JSON(200, res)
}
{{- range .MethodSets}}
	func (Unimplemented{{$svrType}}HTTPServer) {{.Name}}(*gin.Context, *{{.Request}}) (*{{.Reply}}, error) {return nil, errors.New("method not implemented")}
{{- end}}

func Register{{.ServiceType}}HTTPServer(s *http.Server, router *gin.Engine, srv {{.ServiceType}}HTTPServer) {
	{{- range .Methods}}
	router.{{.Method}}("{{.Path}}", _{{$svrType}}_{{.Name}}{{.Num}}_HTTP_Handler(srv))
	{{- end}}
	// redirect with splash
	{{- range .Methods}}
	{{- if not (endWithSplash .Path)}}
	router.{{.Method}}("{{.Path}}/", _{{$svrType}}_{{.Name}}{{.Num}}_HTTP_Handler(srv))
	{{- end }}
	{{- end}}
	s.HandlePrefix("/", router)
}

{{range .Methods}}
func _{{$svrType}}_{{.Name}}{{.Num}}_HTTP_Handler(srv {{$svrType}}HTTPServer) func(c *gin.Context) {
	return func(c *gin.Context) {
		var in {{.Request}}
		b := binding1.Default(c.Request.Method, c.ContentType())
		if b.Name() == "json" {
			if err := c.ShouldBindBodyWith(&in, binding1.JSON); err != nil {
				srv.Err(c, err)
				return
			}
		} else {
			if err := c.ShouldBindWith(&in, b); err != nil {
				srv.Err(c, err)
				return
			}
		}

		reply, err := srv.{{.Name}}(c, &in)
		if err != nil {
			srv.Err(c, err)
			return
		}
		srv.Data(c, reply)
	}
}
{{end}}


type {{.ServiceType}}HTTPClient interface {
{{- range .MethodSets}}
	{{.Name}}(ctx context.Context, req *{{.Request}}, opts ...http.CallOption) (rsp *{{.Reply}}, err error) 
{{- end}}
}
	
type {{.ServiceType}}HTTPClientImpl struct{
	cc *http.Client
}
	
func New{{.ServiceType}}HTTPClient (client *http.Client) {{.ServiceType}}HTTPClient {
	return &{{.ServiceType}}HTTPClientImpl{client}
}

{{range .MethodSets}}
func (c *{{$svrType}}HTTPClientImpl) {{.Name}}(ctx context.Context, in *{{.Request}}, opts ...http.CallOption) (*{{.Reply}}, error) {
	var out {{.Reply}}
	pattern := "{{.Path}}"
	path := binding.EncodeURL(pattern, in, {{not .HasBody}})
	opts = append(opts, http.Operation("/{{$svrName}}/{{.Name}}"))
	opts = append(opts, http.PathTemplate(pattern))
	{{if .HasBody -}}
	err := c.cc.Invoke(ctx, "{{.Method}}", path, in{{.Body}}, &out{{.ResponseBody}}, opts...)
	{{else -}} 
	err := c.cc.Invoke(ctx, "{{.Method}}", path, nil, &out{{.ResponseBody}}, opts...)
	{{end -}}
	if err != nil {
		return nil, err
	}
	return &out, err
}
{{end}}
`

type serviceDesc struct {
	ServiceType string // Greeter
	ServiceName string // helloworld.Greeter
	Metadata    string // api/helloworld/helloworld.proto
	Methods     []*methodDesc
	MethodSets  map[string]*methodDesc
}

type methodDesc struct {
	// method
	Name    string
	Num     int
	Request string
	Reply   string
	// http_rule
	Path         string
	Method       string
	HasVars      bool
	HasBody      bool
	Body         string
	ResponseBody string
}

func (s *serviceDesc) execute() string {
	s.MethodSets = make(map[string]*methodDesc)
	for _, m := range s.Methods {
		s.MethodSets[m.Name] = m
	}
	buf := new(bytes.Buffer)
	tmpl, err := template.New("http").Funcs(
		map[string]interface{}{
			"endWithSplash": EndWithSlash,
		},
	).Parse(strings.TrimSpace(httpTemplate))
	if err != nil {
		panic(err)
	}
	if err := tmpl.Execute(buf, s); err != nil {
		panic(err)
	}
	return strings.Trim(buf.String(), "\r\n")
}

func EndWithSlash(s string) bool {
	if strings.HasSuffix(s, "/") {
		return true
	}
	return false
}
