"""
TIR Test Suite Runner for SC5
"""
import unittest

from SC5TESTCASE import SC5

suite = unittest.TestSuite()
suite.addTest(SC5('test_SC5_CT001'))
suite.addTest(SC5('test_SC5_CT002'))

runner = unittest.TextTestRunner(verbosity=2)
runner.run(suite)
