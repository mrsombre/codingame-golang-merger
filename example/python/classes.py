from dataclasses import dataclass


@dataclass
class ExampleType:
    name: str
    value: int


@dataclass
class ExampleUnusedType:
    id: int


class ExampleTypeWithFunc:
    def __init__(self, id: int, value: float):
        self.id = id
        self.value = value

    def read(self) -> float:
        return self._calibrate()

    def _calibrate(self) -> float:
        return self.value * 1.05


class ExampleTypeUsingOther:
    def __init__(self, name: str):
        self.name = name

    def check(self, s: ExampleTypeWithFunc) -> str:
        val = s.read()
        if val > 100:
            return f"{self.name}: HIGH"
        return f"{self.name}: OK"
