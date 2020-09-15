package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func calcIndent(path string, isPrevIndent string, isPrevLast bool) string {
	indent := ""
	sepsNumber := strings.Count(path, string(os.PathSeparator))
	if sepsNumber < 2 {
		return indent
	}

	if isPrevLast {
		indent = isPrevIndent + "\t"
	} else {
		indent = isPrevIndent + "│\t"
	}

	return indent
}

func filterFiles(objsInfo []os.FileInfo) []os.FileInfo {
	tmpObjsInfo := make([]os.FileInfo, 0)
	for _, objInfo := range objsInfo {
		if objInfo.IsDir() {
			tmpObjsInfo = append(tmpObjsInfo, objInfo)
		}
	}

	return tmpObjsInfo
}

func getObjName(path string) string {
	separatorLastIndex := strings.LastIndex(path, string(os.PathSeparator))
	if separatorLastIndex == -1 {
		return ""
	}

	return path[strings.LastIndex(path, string(os.PathSeparator))+1:]
}

func getFileSize(fileInfo os.FileInfo) string {
	if fileInfo.Size() == 0 {
		return " (empty)"
	}

	return fmt.Sprintf(" (%db)", fileInfo.Size())
}

func printTree(out *bytes.Buffer, path string, prevIndent string, printFiles bool, isLast bool, isPrevLast bool) error {
	objStat, err := os.Stat(path)
	if err != nil {
		return err
	}

	currIndent := calcIndent(path, prevIndent, isPrevLast)
	prevIndent = currIndent
	if isLast {
		currIndent += "└───"
	} else {
		currIndent += "├───"
	}

	if !objStat.IsDir() {
		if printFiles {
			out.WriteString(currIndent + getObjName(path) + getFileSize(objStat) + "\n")
		}
	} else {
		if dirName := getObjName(path); dirName != "" {
			out.WriteString(currIndent + dirName + "\n")
		}

		innerObjs, err := ioutil.ReadDir(path)
		if err != nil {
			return err
		}
		if !printFiles {
			innerObjs = filterFiles(innerObjs)
		}

		isCurrLast := false
		for innerObjIdx, innerObj := range innerObjs {
			if innerObjIdx == len(innerObjs)-1 {
				isCurrLast = true
			}
			printTree(out, path+"/"+innerObj.Name(), prevIndent, printFiles, isCurrLast, isLast)
		}
	}

	return nil
}

func dirTree(out *bytes.Buffer, path string, printFiles bool) error {
	printTree(out, path, "", printFiles, false, false)
	return nil
}

func main() {
	out := new(bytes.Buffer)
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(out)
}
