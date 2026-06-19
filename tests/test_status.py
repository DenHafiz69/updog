from metrics import Metrics
from status import check_status


def test_check_status():

    # Normal test cases
    test1 = Metrics("CPU", 60, 80, 90)
    test2 = Metrics("CPU", 85, 80, 90)
    test3 = Metrics("CPU", 91, 80, 90)

    # Edge cases
    test4 = Metrics("RAM", 80, 80, 90)
    test5 = Metrics("RAM", 90, 80, 90)
    test6 = Metrics("RAM", 90.1, 80, 90)

    assert check_status(test1) == "healthy"
    assert check_status(test2) == "warning"
    assert check_status(test3) == "critical"
    assert check_status(test4) == "healthy"
    assert check_status(test5) == "warning"
    assert check_status(test6) == "critical"
