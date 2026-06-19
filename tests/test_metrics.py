from metrics import Metrics


def test_status():

    tests: list[tuple[Metrics, int]] = [
        (Metrics("CPU", 70, 80, 90), 0),
        (Metrics("CPU", 85, 80, 90), 1),
        (Metrics("CPU", 92, 80, 90), 2),
    ]

    for test in tests:
        assert test[0].check_status() == test[1]
