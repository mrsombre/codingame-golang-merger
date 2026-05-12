import sys
from funcs import example_func
from classes import ExampleType, ExampleTypeWithFunc, ExampleTypeUsingOther
from vars import EXAMPLE_CONST, EXAMPLE_VAR, EXAMPLE_MULTI_VAR_A, EXAMPLE_MULTI_VAR_B


def main():
    v = example_func(1, 2)
    print(v)
    print(EXAMPLE_CONST)
    print(EXAMPLE_VAR)
    print(EXAMPLE_MULTI_VAR_A, EXAMPLE_MULTI_VAR_B)

    s = ExampleType(name="test", value=42)
    print(s.name, s.value)

    ex_with_func = ExampleTypeWithFunc(id=1, value=95.0)
    ex_using_other = ExampleTypeUsingOther(name="main")
    print(ex_using_other.check(ex_with_func))


if __name__ == "__main__":
    main()
