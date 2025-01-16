package testing

import (
	"fmt"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func BeNotFoundError() types.GomegaMatcher {
	return BeAPIError(NotFoundError)
}

func BeForbiddenError() types.GomegaMatcher {
	return BeAPIError(ForbiddenError)
}

func BeInvalidError() types.GomegaMatcher {
	return BeAPIError(InvalidError)
}

type errorMatcher int

const (
	NotFoundError errorMatcher = iota
	ForbiddenError
	InvalidError
)

func (em errorMatcher) String() string {
	return []string{"NotFoundError", "ForbiddenError", "InvalidError"}[em]
}

type apiError func(error) bool

func (em errorMatcher) isAPIError(err error) bool {
	return []apiError{apierrors.IsNotFound, apierrors.IsForbidden, apierrors.IsInvalid}[em](err)
}

type isErrorMatch struct {
	name errorMatcher
}

func BeAPIError(name errorMatcher) types.GomegaMatcher {
	return &isErrorMatch{
		name: name,
	}
}

func (matcher *isErrorMatch) Match(actual interface{}) (success bool, err error) {
	err, ok := actual.(error)
	if !ok {
		return false, fmt.Errorf("%s expects an error", matcher.name.String())
	}

	return err != nil && matcher.name.isAPIError(err), nil
}

func (matcher *isErrorMatch) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to be a %s", matcher.name.String())
}

func (matcher *isErrorMatch) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to be %s", matcher.name.String())
}
