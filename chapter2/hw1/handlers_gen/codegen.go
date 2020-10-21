package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

var (
	genKeyword = "apigen:api "
	valKeyword = "apivalidator:"

	srvs = make(map[string][]*Method)
)

// Server ...
type Server struct {
	Name     string
	Handlers map[string]string
}

// Method ...
type Method struct {
	Name   string   `json:"-"`
	Recv   string   `json:"-"`
	Params []string `json:"-"`
	URL    string   `json:"url"`
	Auth   bool     `json:"auth"`
	Method string   `json:"method"`
}

// Field ...
type Field struct {
	Name string
	Type string
	Tag  string
}

// Struct ...
type Struct struct {
	Name   string
	Fields []Field
}

func parseFile(path string) (*ast.File, error) {
	// Read file
	inputFileRaw, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	inputFile := string(inputFileRaw)

	// Parse file
	parsedFile, err := parser.ParseFile(token.NewFileSet(), "", inputFile, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	return parsedFile, nil
}

func fillMethod(funcDecl *ast.FuncDecl) *Method {
	method := &Method{}
	method.Name = funcDecl.Name.String()

	recv := ""
	if funcDecl.Recv != nil {
		if starExpr, isOk := funcDecl.Recv.List[0].Type.(*ast.StarExpr); isOk {
			recv = starExpr.X.(*ast.Ident).String()
		}
	}
	method.Recv = recv

	for _, param := range funcDecl.Type.Params.List {
		if param, isOk := param.Type.(*ast.Ident); isOk {
			method.Params = append(method.Params, param.String())
		}
	}

	funcComment := funcDecl.Doc.Text()
	err := json.Unmarshal([]byte(strings.TrimPrefix(funcComment, "apigen:api ")), &method)
	if err != nil {
		return nil
	}

	return method
}

func fillStruct(g *ast.GenDecl) *Struct {
	if len(g.Specs) == 0 {
		return nil
	}

	filledStruct := &Struct{}
	filledStruct = nil

	if typeSpec, isOk := g.Specs[0].(*ast.TypeSpec); isOk {
		if structType, isOk := typeSpec.Type.(*ast.StructType); isOk {
			filledStruct = &Struct{
				Name: typeSpec.Name.String(),
			}

			if structType.Fields != nil {
				for _, field := range structType.Fields.List {
					tagValue := ""
					if field.Tag != nil {
						tagValue = field.Tag.Value
					}
					typeName := ""
					if typeIdent, isOk := field.Type.(*ast.Ident); isOk {
						typeName = typeIdent.String()
					}

					filledStruct.Fields = append(filledStruct.Fields, Field{
						Name: field.Names[0].String(),
						Type: typeName,
						Tag:  tagValue,
					})
				}
			}
		}
	}

	return filledStruct
}

func generateHandler(w io.Writer, f *ast.FuncDecl) {
	method := fillMethod(f)
	if method == nil {
		return
	}
	srvs[method.Recv] = append(srvs[method.Recv], method)

	methodHandler := fmt.Sprintf("func (s *%s) handle%s(w http.ResponseWriter, r *http.Request) {\n", method.Recv, method.Name)
	if method.Method != "" {
		methodHandler += fmt.Sprintf(`	if r.Method != "%s" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("{\"error\": \"bad method\"}"))
	}
	
`, method.Method)
	}

	methodHandler += fmt.Sprintf(`	params, err := validate%s(r.URL.Query())
	if err != nil {
		errJSON := fmt.Sprintf("{\"error\": \"%s\"}", err)
		errRaw, _ := json.Marshal(errJSON)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(errRaw)

		return
	}

	ctx := context.Background()
	res, err := s.%s(ctx, params)
	if err != nil {
		errJSON := fmt.Sprintf("{\"error\": \"%s\"}", err)
		errRaw, _ := json.Marshal(errJSON)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(errRaw)

		return
	}

	resRaw, _ := json.Marshal(res)
	w.WriteHeader(http.StatusOK)
	w.Write(resRaw)
}

`, method.Params[0], "%s", method.Name, "%s")

	fmt.Fprint(w, methodHandler)
}

func generateValidator(w io.Writer, g *ast.GenDecl) {
	s := fillStruct(g)
	if s == nil {
		return
	}

	getSplittedTags := func(tag string) []string {
		bParamsPos := strings.Index(tag, `"`) + 1
		eParamsPos := strings.LastIndex(tag, `"`)

		return strings.Split(tag[bParamsPos:eParamsPos], ",")
	}

	funcBody := ""
	for _, field := range s.Fields {
		if strings.Contains(field.Tag, valKeyword) {
			funcBody += fmt.Sprintf("\t// %s\n", field.Name)
			tagVals := make(map[string][]string)
			splittedTags := getSplittedTags(field.Tag)
			for _, tag := range splittedTags {
				tagVal := ""
				splittedTag := strings.Split(tag, "=")
				if len(splittedTag) > 1 {
					tagVal = splittedTag[1]
				}
				tagName := splittedTag[0]
				tagVals[tagName] = strings.Split(tagVal, "|")
			}

			// paramname
			paramName := strings.ToLower(field.Name)
			if tagVal, isExist := tagVals["paramname"]; isExist {
				paramName = tagVal[0]
			}

			if field.Type == "string" {
				funcBody += fmt.Sprintf("\tif params, isExist := q[\"%s\"]; isExist {\n\t\tp.%s = params[0];\n\t}\n\n", paramName, field.Name)
			} else if field.Type == "int" {
				funcBody += fmt.Sprintf("\tif params, isExist := q[\"%s\"]; isExist {\n", paramName)
				funcBody += "\t\tn, err := strconv.Atoi(params[0])\n\t\tif err != nil{\n"
				funcBody += fmt.Sprintf("\t\t\treturn p, errors.New(\"%s must be int\")\n\t\t}\n\n", paramName)
				funcBody += fmt.Sprintf("\t\tp.%s = n\n\t}\n\n", field.Name)
			}

			// default
			if tagVal, isExist := tagVals["default"]; isExist {
				if field.Type == "string" {
					funcBody += fmt.Sprintf("\tif p.%s == \"\" {\n\t\tp.%s = \"%s\"\n\t}\n\n", field.Name, field.Name, tagVal[0])
				} else if field.Type == "int" {
					funcBody += fmt.Sprintf("\tif p.%s == 0 {\n\t\tp.%s = %s\n\t}\n\n", field.Name, field.Name, tagVal[0])
				}
			}

			// required
			if _, isExist := tagVals["required"]; isExist {
				if field.Type == "string" {
					funcBody += fmt.Sprintf("\tif p.%s == \"\" {\n\t\treturn p, errors.New(\"%s must me not empty\")\n\t}\n\n", field.Name, paramName)
				} else if field.Type == "int" {
					funcBody += fmt.Sprintf("\tif p.%s == 0 {\n\t\treturn p, errors.New(\"%s must me not 0\")\n\t}\n\n", field.Name, paramName)
				}
			}

			// enum
			if tagVal, isExist := tagVals["enum"]; isExist {
				if field.Type == "string" {
					funcBody += "\tisAllowed := false\n\tallowedVals := []string{"
					sliceVals := []string{}
					for _, val := range tagVal {
						sliceVals = append(sliceVals, "\""+val+"\"")
					}
					funcBody += strings.Join(sliceVals, ", ") + "}\n"
				} else if field.Type == "int" {
					funcBody += "\tisAllowed := false\n\tallowedVals := []int{"
					funcBody += strings.Join(tagVal, ", ") + "}\n"
				}
				funcBody += "\tfor _, val := range allowedVals {\n"
				funcBody += fmt.Sprintf("\t\tif p.%s == val {\n\t\t\tisAllowed = true\n\t\t\tbreak\n\t\t}\n\t}\n", field.Name)
				funcBody += fmt.Sprintf("\tif !isAllowed {\n\t\treturn p, errors.New(\"%s must be one of ", paramName)
				funcBody += "[" + strings.Join(tagVal, ", ") + "]\")\n\t}\n\n"
			}

			// min
			if tagVal, isExist := tagVals["min"]; isExist {
				if field.Type == "string" {
					funcBody += fmt.Sprintf("\tif len(p.%s) < %s {\n\t\treturn p, errors.New(\"%s len must be >= %s\")\n\t}\n\n", field.Name, tagVal[0], paramName, tagVal[0])
				} else if field.Type == "int" {
					funcBody += fmt.Sprintf("\tif p.%s < %s {\n\t\treturn p, errors.New(\"%s must be >= %s\")\n\t}\n\n", field.Name, tagVal[0], paramName, tagVal[0])
				}
			}

			// max
			if tagVal, isExist := tagVals["max"]; isExist {
				if field.Type == "string" {
					funcBody += fmt.Sprintf("\tif len(p.%s) > %s {\n\t\treturn p, errors.New(\"%s len must be <= %s\")\n\t}\n\n", field.Name, tagVal[0], paramName, tagVal[0])
				} else if field.Type == "int" {
					funcBody += fmt.Sprintf("\tif p.%s > %s {\n\t\treturn p, errors.New(\"%s must be <= %s\")\n\t}\n\n", field.Name, tagVal[0], paramName, tagVal[0])
				}
			}
		}
	}

	if funcBody != "" {
		fmt.Fprintf(w, "func validate%s(q map[string][]string,) (%s, error) {\n\tp := %s{}\n\n%s\treturn p, nil\n}\n\n", s.Name, s.Name, s.Name, funcBody)
	}
}

func generateSrvs(w io.Writer, srvs map[string][]*Method) {
	for srvName, srvMethods := range srvs {
		fmt.Fprintf(w, "func (s *%s) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n", srvName)
		if len(srvMethods) != 0 {
			fmt.Fprint(w, `	switch r.URL.Path {`)
			for _, method := range srvMethods {
				fmt.Fprintf(w, `
	case "%s":
		s.handle%s(w, r)`, method.URL, method.Name)
			}
			fmt.Fprint(w, `
	default:
		errRaw, _ := json.Marshal("{\"error\": \"unknown method\"}")
		w.WriteHeader(http.StatusNotFound)
		w.Write(errRaw)
	}
}

`)
		}
	}
}

func generateAPI(inputFilePath string, outputFilePath string) error {
	// parse input file
	parsedFile, err := parseFile(inputFilePath)
	if err != nil {
		return err
	}

	// create output file
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// write package name
	_, err = fmt.Fprint(outputFile, "package "+parsedFile.Name.String()+"\n\n")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(outputFile, "import (\n\t\"context\"\n\t\"encoding/json\"\n\t\"errors\"\n\t\"fmt\"\n\t\"net/http\"\n\t\"strconv\"\n)\n\n")
	if err != nil {
		return err
	}

	// write handlers for methods and validators for structs
	for _, decl := range parsedFile.Decls {
		switch decl.(type) {
		case *ast.FuncDecl:
			funcDecl, _ := decl.(*ast.FuncDecl)
			generateHandler(outputFile, funcDecl)
		case *ast.GenDecl:
			genDecl, _ := decl.(*ast.GenDecl)
			generateValidator(outputFile, genDecl)
		}
	}

	// write servers
	generateSrvs(outputFile, srvs)

	return nil
}

func main() {
	generateAPI(os.Args[1], os.Args[2])
}
