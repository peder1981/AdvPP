"""
TIR Test Suite Runner for SA1
"""
import unittest

from SA1TESTCASE import SA1

suite = unittest.TestSuite()

suite.addTest(SA1('test_SA1_CT001'))
suite.addTest(SA1('test_SA1_CT002'))
suite.addTest(SA1('test_SA1_CT003'))
suite.addTest(SA1('test_SA1_CT004'))

runner = unittest.TextTestRunner(verbosity=2)
runner.run(suite)
