package main

import (
	"fmt"
	"reflect"
)

func i2s(in interface{}, out interface{}) error {
	inValueOf := reflect.ValueOf(in)
	outValueOf := reflect.ValueOf(out)

	if outValueOf.Kind() != reflect.Ptr {
		return fmt.Errorf("invalid type")
	}

	switch inValueOf.Kind() {
	case reflect.Map:
		if outValueOf.Elem().Type().Kind() != reflect.Struct {
			return fmt.Errorf("invalid type")
		}

		inAsAMap := in.(map[string]interface{})
		for inFieldName, inFieldVal := range inAsAMap {
			outFiedVal := outValueOf.Elem().FieldByName(inFieldName)
			if outFiedVal.IsValid() && outFiedVal.CanSet() {
				switch outFiedVal.Kind() {
				case reflect.Int:
					inFieldValFloat, isOk := inFieldVal.(float64)
					if !isOk {
						return fmt.Errorf("invalid type")
					}
					outFiedVal.SetInt(int64(inFieldValFloat))
				case reflect.String:
					inFieldValStr, isOk := inFieldVal.(string)
					if !isOk {
						return fmt.Errorf("invalid type")
					}
					outFiedVal.Set(reflect.ValueOf(inFieldValStr))
				case reflect.Bool:
					inFieldValBool, isOk := inFieldVal.(bool)
					if !isOk {
						return fmt.Errorf("invalid type")
					}
					outFiedVal.Set(reflect.ValueOf(inFieldValBool))
				default:
					err := i2s(inFieldVal, outFiedVal.Addr().Interface())
					if err != nil {
						return err
					}
				}
			}
		}
	case reflect.Slice:
		if outValueOf.Elem().Type().Kind() != reflect.Slice {
			return fmt.Errorf("invalid type")
		}

		inAsASlice := in.([]interface{})
		for idx := 0; idx < len(inAsASlice); idx++ {
			outElem := reflect.New(reflect.TypeOf(out).Elem().Elem())
			err := i2s(inAsASlice[idx], outElem.Interface())
			if err != nil {
				return err
			}

			outValueOf.Elem().Set(reflect.Append(outValueOf.Elem(), outElem.Elem()))
		}
	default:
		return fmt.Errorf("invalid type")
	}

	return nil
}
