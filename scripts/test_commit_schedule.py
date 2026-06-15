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
        total = schedule.THREE_SLOT_DAYS * 3 + schedule.ACTIVE_DAYS - schedule.THREE_SLOT_DAYS
        slots = [schedule.slot(i) for i in range(total)]
        self.assertTrue(all(a < b for a, b in zip(slots, slots[1:])))
        self.assertTrue(all(schedule.START <= s.date() <= schedule.END for s in slots))

    def test_refuses_exhaustion(self):
        total = schedule.THREE_SLOT_DAYS * 3 + schedule.ACTIVE_DAYS - schedule.THREE_SLOT_DAYS
        for index in (-1, total):
            with self.assertRaises(ValueError):
                schedule.slot(index)

    def test_randomized_times_are_stable_and_spaced(self):
        for day in schedule.active_days()[3:]:
            seconds = schedule.day_seconds(day)
            self.assertEqual(seconds, schedule.day_seconds(day))
            self.assertEqual(len(set(seconds)), 3)
            self.assertTrue(all(8 * 3600 <= second < 23 * 3600 for second in seconds))
            self.assertTrue(all(b - a >= 1200 for a, b in zip(seconds, seconds[1:])))
        total = schedule.THREE_SLOT_DAYS * 3 + schedule.ACTIVE_DAYS - schedule.THREE_SLOT_DAYS
        times = {schedule.slot(i).time() for i in range(9, total)}
        self.assertGreater(len(times), 250)
        self.assertTrue(any(t.second != 0 for t in times))

    def test_existing_nine_slots_unchanged(self):
        for i in range(9):
            self.assertEqual(schedule.slot(i).hour, (9, 13, 17)[i % 3])
            self.assertEqual(schedule.slot(i).minute, 0)
            self.assertEqual(schedule.slot(i).second, 0)


if __name__ == "__main__":
    unittest.main()
