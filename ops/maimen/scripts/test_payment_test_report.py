import json
import unittest

from check_payment_test_report import check, PACKAGE, REQUIRED


def events(*items):
    return [json.dumps(item) + "\n" for item in items]


def success():
    return events(
        *({"Action": "pass", "Package": PACKAGE, "Test": name} for name in REQUIRED),
        {"Action": "pass", "Package": PACKAGE},
    )


class PaymentReportTests(unittest.TestCase):
    def test_complete_database_run_is_accepted(self):
        check(success())

    def test_green_run_without_required_tests_is_rejected(self):
        for report in [[], events({"Action": "pass", "Package": PACKAGE}), success()[:-1]]:
            with self.subTest(report=report), self.assertRaises(ValueError):
                check(report)

    def test_required_test_or_subtest_skip_is_rejected(self):
        for name in REQUIRED:
            for suffix in ["", "/nested"]:
                with self.subTest(name=name, suffix=suffix), self.assertRaises(ValueError):
                    check(success() + events({"Action": "skip", "Package": PACKAGE, "Test": name + suffix}))

    def test_other_package_failure_cannot_be_hidden_by_database_success(self):
        with self.assertRaises(ValueError):
            check(success() + events({"Action": "fail", "Package": "another/package"}))

    def test_wrong_package_names_and_truncated_output_are_rejected(self):
        for report in [[line.replace(PACKAGE, "wrong/package") for line in success()], success() + ['{"Action":']]:
            with self.subTest(report=report), self.assertRaises(ValueError):
                check(report)


if __name__ == "__main__":
    unittest.main()
