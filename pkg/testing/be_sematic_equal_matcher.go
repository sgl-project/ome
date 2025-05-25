package testing

import (
	"fmt"
	"reflect"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/api/equality"
)

// Gomega does not support semantic equal so adding this here in testing util

func BeSematicEqual(expected interface{}) types.GomegaMatcher {
	return &BeSematicEqualMatcher{
		Expected: expected,
	}
}

type BeSematicEqualMatcher struct {
	Expected interface{}
}

func (matcher *BeSematicEqualMatcher) Match(actual interface{}) (success bool, err error) {
	if actual == nil && matcher.Expected == nil {
		return false, fmt.Errorf("Both actual and expected must not be nil.")
	}

	convertedActual := actual

	if actual != nil && matcher.Expected != nil && reflect.TypeOf(actual).ConvertibleTo(reflect.TypeOf(matcher.Expected)) {
		convertedActual = reflect.ValueOf(actual).Convert(reflect.TypeOf(matcher.Expected)).Interface()
	}

	return equality.Semantic.DeepEqual(convertedActual, matcher.Expected), nil
}

func (matcher *BeSematicEqualMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to be equivalent to", matcher.Expected)
}

func (matcher *BeSematicEqualMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to be equivalent to", matcher.Expected)
}
