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

	handlers   = make(map[string]string)
	validators = make(map[string]string)
)

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

	fmt.Fprintf(w, `func (s *%s) handle%s(w http.Response, r *http.Request) {
	// Fill params
	// Validate

	ctx := context.Background()
	res, err := s.%s(ctx, params)
}

`, method.Recv, method.Name, method.Name)
}

func generateValidator(w io.Writer, g *ast.GenDecl) {
	s := fillStruct(g)
	if s == nil {
		return
	}

	funcBody := ""
	for _, field := range s.Fields {
		if strings.Contains(field.Tag, valKeyword) {
			paramName := strings.ToLower(field.Name)
			pureTags := field.Tag[strings.Index(field.Tag, "\"")+1 : strings.LastIndex(field.Tag, "\"")]
			for _, tag := range strings.Split(pureTags, ",") {
				if field.Type == "string" {
					tagName := strings.Split(tag, "=")[0]
					switch tagName {
					case "required":
						funcBody += fmt.Sprintf(`	if _, isExist := q["%s"]; !isExist {
		return errors.New("%s must me not empty")
	}
`, paramName, paramName)
						funcBody += fmt.Sprintf("\tp.%s = q[\"%s\"][0]\n\n", field.Name, paramName)
					case "paramname":
						paramName = strings.Split(tag, "=")[1]
					case "enum":
						funcBody += "\tisAllowed := false\n\tallowedVals := []string{"
						allowedVals := strings.Split(strings.Split(tag, "=")[1], "|")
						for valIdx, val := range allowedVals {
							comma := ","
							if len(allowedVals)-1 == valIdx {
								comma = "}"
							}
							funcBody += "\"" + val + "\"" + comma
						}
						funcBody += "\n"

						funcBody += "\tfor _, val := range allowedVals {\n"
						funcBody += fmt.Sprintf(`		if q["%s"][0] == val {
			p.%s = val
			isAllowed = true
			break
		}
	}
`, paramName, field.Name)

						funcBody += fmt.Sprintf("\tif !isAllowed {\n\t\treturn errors.New(\"%s must be one of ", paramName)
						funcBody += "[" + strings.Join(allowedVals, ", ") + "]\")\n\t}\n\n"
					case "default":
						funcBody += fmt.Sprintf("\tp.%s = q[\"%s\"][0]\n", field.Name, paramName)
						funcBody += fmt.Sprintf(`	if p.%s == "" {
		p.%s = "%s"
	}`, field.Name, field.Name, strings.Split(tag, "=")[1])
					case "min":

					default:
						continue
					}
				} else if field.Type == "int" {

				}
			}
		}
	}

	if funcBody != "" {
		fmt.Fprintf(w, `func validate%s(q map[string][]string, p *%s) error {
%s

	return nil
}

`, s.Name, s.Name, funcBody)

		validators[s.Name] = "validate" + s.Name
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

	_, err = fmt.Fprintf(outputFile, "import (\n\t\"context\"\n\t\"errors\"\n\t\"net/http\"\n)\n\n")
	if err != nil {
		return err
	}

	// write handlers for methods and validators for structs
	for _, decl := range parsedFile.Decls {
		if funcDecl, isOk := decl.(*ast.FuncDecl); isOk {
			generateHandler(outputFile, funcDecl)
		} else if genDecl, isOk := decl.(*ast.GenDecl); isOk {
			generateValidator(outputFile, genDecl)
		}
	}

	return nil
}

func main() {
	generateAPI(os.Args[1], os.Args[2])
}
