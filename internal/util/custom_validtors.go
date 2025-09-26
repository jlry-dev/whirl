package util

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

/*
This was supposed to help me validate if there is a space from a string

but I found out validtor already has a containsrune validator.
*/
func NoSpaceValidator(fl validator.FieldLevel) bool {
	v, k, _ := fl.ExtractType(fl.Field())

	if k != reflect.String {
		panic("no space validator only works on strings")
	}

	str := v.String()
	if strings.ContainsRune(str, ' ') {
		return false
	}

	return true
}

func ValidAgeValidator(fl validator.FieldLevel) bool {
	v := fl.Field().String()
	p, err := strconv.Atoi(fl.Param())
	if err != nil {
		return false
	}

	bdate, err := time.Parse("2006-01-02", v)
	if err != nil {
		return false
	}

	ct := time.Now()

	if bdate.After(ct) {
		return false
	}

	year := ct.Year() - bdate.Year()
	// Valid age is 13, change as required
	if year < p {
		return false
	}

	return true
}

/*
This checks if the format is YYYY-MM-DD is followed

# If field value is YYYY-M-D it will set it to YYYY-MM-DD

If date values are invalid like Month being 13 or Days more than what a month has it will fail
*/
func DateFormatValidator(fl validator.FieldLevel) bool {
	v := fl.Field().String()
	if f, err := time.Parse("2006-1-2", v); err == nil {
		// it parse correctly
		// so we correct the format and set it the field value
		newForm := f.Format(time.DateOnly)
		fl.Field().SetString(newForm)
		return true
	}

	// the first check might fail, so we need to check also if it's already in the correct format
	_, err := time.Parse(time.DateOnly, v)
	return err == nil
}
