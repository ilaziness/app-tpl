// Package validator provides request validation with multi-language support.
package validator

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"
)

var (
	validate   *validator.Validate
	translator ut.Translator
)

func init() {
	// Use gin's default validator engine (SetTagName("binding")).
	engine := binding.Validator.Engine()
	v, ok := engine.(*validator.Validate)
	if !ok {
		panic(fmt.Sprintf("expected *validator.Validate, got %T", engine))
	}
	validate = v

	zhLocale := zh.New()
	uni := ut.New(zhLocale, zhLocale)
	translator, _ = uni.GetTranslator("zh")

	_ = zhTranslations.RegisterDefaultTranslations(validate, translator)

	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			return field.Name
		}
		if idx := strings.Index(jsonTag, ","); idx != -1 {
			jsonTag = jsonTag[:idx]
		}
		return jsonTag
	})

	_ = validate.RegisterValidation("search", searchKeyword)
	_ = RegisterTranslation("search", "{0}只能包含中英文、数字及常见文本符号")
}

// Validate validates a struct and returns translated error messages.
func Validate(s any) error {
	return TranslateError(validate.Struct(s))
}

// TranslateError translates validator errors into Chinese messages.
// If the error is not a validation error, it is returned as-is.
func TranslateError(err error) error {
	if err == nil {
		return nil
	}
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		messages := make([]string, 0, len(validationErrors))
		for _, e := range validationErrors {
			messages = append(messages, e.Translate(translator))
		}
		return fmt.Errorf("%s", strings.Join(messages, "; "))
	}
	var sliceErr binding.SliceValidationError
	if errors.As(err, &sliceErr) {
		if len(sliceErr) == 0 {
			return nil
		}
		messages := make([]string, 0, len(sliceErr))
		for _, e := range sliceErr {
			if te := TranslateError(e); te != nil {
				messages = append(messages, te.Error())
			}
		}
		if len(messages) == 0 {
			return nil
		}
		return fmt.Errorf("%s", strings.Join(messages, "; "))
	}
	return err
}

// ValidateVar validates a single variable with given tag.
func ValidateVar(field any, tag string) error {
	if err := validate.Var(field, tag); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			fe := validationErrors[0]
			return fmt.Errorf("%s (%s)", fe.Translate(translator), fe.Tag())
		}
		return err
	}
	return nil
}

// GetValidator returns the underlying validator instance.
func GetValidator() *validator.Validate {
	return validate
}

// GetTranslator returns the translator instance.
func GetTranslator() ut.Translator {
	return translator
}

// RegisterValidation registers a custom validation function.
func RegisterValidation(tag string, fn validator.Func, callValidationEvenIfNull ...bool) error {
	return validate.RegisterValidation(tag, fn, callValidationEvenIfNull...)
}

// RegisterTranslation registers a custom translation for a validation tag.
func RegisterTranslation(tag string, message string) error {
	return validate.RegisterTranslation(tag, translator, func(ut ut.Translator) error {
		return ut.Add(tag, message, true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		return fe.Translate(ut)
	})
}

// searchKeyword validates that a keyword contains only Chinese/English characters,
// numbers, spaces and common Chinese/English punctuation.
func searchKeyword(fl validator.FieldLevel) bool {
	v := fl.Field().String()
	allowed := " .,!?;:'\"()[]{}@#&*_-~。，、；：？！…—·（）【】《》“”‘’"
	for _, r := range v {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Latin, r) || unicode.IsNumber(r) || r == ' ' || strings.ContainsRune(allowed, r) {
			continue
		}
		return false
	}
	return true
}
