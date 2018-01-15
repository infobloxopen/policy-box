package pdp

import "fmt"

type functionFloatDivide struct {
	first  Expression
	second Expression
}

func makeFunctionFloatDivide(first, second Expression) Expression {
	return functionFloatDivide{
		first:  first,
		second: second,
	}
}

func makeFunctionFloatDivideAlt(args []Expression) Expression {
	if len(args) != 2 {
		panic(fmt.Errorf("function \"divide\" for Float needs exactly two arguments but got %d", len(args)))
	}

	return makeFunctionFloatDivide(args[0], args[1])
}

func (f functionFloatDivide) GetResultType() int {
	return TypeFloat
}

func (f functionFloatDivide) calculate(ctx *Context) (AttributeValue, error) {
	first, err := ctx.calculateFloatOrIntegerExpression(f.first)
	if err != nil {
		return undefinedValue, bindError(bindError(err, "first argument"), "equal")
	}

	second, err := ctx.calculateFloatOrIntegerExpression(f.second)
	if err != nil {
		return undefinedValue, bindError(bindError(err, "second argument"), "equal")
	}

	return MakeFloatValue(first / second), nil
}

func functionFloatDivideValidator(args []Expression) functionMaker {
	if len(args) != 2 ||
		(args[0].GetResultType() != TypeFloat && args[0].GetResultType() != TypeInteger) ||
		(args[1].GetResultType() != TypeFloat && args[1].GetResultType() != TypeInteger) {
		return nil
	}
	return makeFunctionFloatDivideAlt
}