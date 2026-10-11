"""
TIR E2E Tests for SC5 (Notas Fiscais)
Module: MATA
Tables: SC5
"""
from tir import Webapp
import unittest

class SC5(unittest.TestCase):
    """
    TIR E2E Tests for SC5 - Notas Fiscais
    Module: MATA
    Tables: SC5
    """

    @classmethod
    def setUpClass(inst):
        inst.oHelper = Webapp()
        inst.oHelper.Setup('MATA', '20260929', 'T1', '01')
        inst.oHelper.Program('SC5')

    def test_SC5_CT001(self):
        """CT001 — Visualizar NF existente"""
        self.oHelper.SearchBrowse('0101', key='Filial+numero')
        self.oHelper.SetButton('Visualizar')
        self.oHelper.CheckResult('C5_NUM', '01')
        self.oHelper.SetButton('Fechar')
        self.oHelper.AssertTrue()

    def test_SC5_CT002(self):
        """CT002 — Impression relatorio"""
        self.oHelper.SetButton('Imprimir')
        self.oHelper.SetButton('OK')
        self.oHelper.WaitProcessing('Processando')
        self.oHelper.AssertTrue()

    @classmethod
    def tearDownClass(inst):
        inst.oHelper.TearDown()

if __name__ == '__main__':
    unittest.main()
