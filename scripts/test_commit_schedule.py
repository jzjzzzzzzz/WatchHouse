import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("schedule", Path(__file__).with_name("commit-step.py"))
schedule = importlib.util.module_from_spec(spec)
spec.loader.exec_module(schedule)


class ScheduleTests(unittest.TestCase):
    def test_coverage(self):
        days = schedule.active_days()
        self.assertEqual(len(set(days)), 163)
        self.assertEqual(days[0], schedule.START)
        self.assertEqual(days[-1], schedule.END)
        self.assertEqual((schedule.END - schedule.START).days + 1, 181)

    def test_slots_are_strictly_increasing(self):
        slots = [schedule.slot(i) for i in range(489)]
        self.assertTrue(all(a < b for a, b in zip(slots, slots[1:])))
        self.assertTrue(all(schedule.START <= s.date() <= schedule.END for s in slots))

    def test_refuses_exhaustion(self):
        for index in (-1, 489):
            with self.assertRaises(ValueError):
                schedule.slot(index)


if __name__ == "__main__":
    unittest.main()
