"""
TIR E2E Tests for SA1 (Cadastro de Clientes)
Module: MATA
Tables: SA1
"""
from tir import Webapp
import unittest

class SA1(unittest.TestCase):
    """
    TIR E2E Tests for SA1 - Clientes
    Module: MATA
    Tables: SA1
    """

    @classmethod
    def setUpClass(inst):
        """Setup: Initialize Webapp and open SA1 routine"""
        inst.oHelper = Webapp()
        inst.oHelper.Setup('MATA', '20260929', 'T1', '01')
        inst.oHelper.Program('SA1')

    def test_SA1_CT001(self):
        """CT001 — Incluir cliente com campos obrigatorios"""
        self.oHelper.SetButton('Incluir')
        self.oHelper.SetBranch('01')
        self.oHelper.SetValue('Codigo', 'TEST01')
        self.oHelper.SetValue('Nome', 'Cliente Teste TIR')
        self.oHelper.SetButton('Salvar')
        self.oHelper.SearchBrowse('01TEST01', key='Filial+codigo')
        self.oHelper.SetButton('Visualizar')
        self.oHelper.CheckResult('A1_COD', 'TEST01')
        self.oHelper.CheckResult('A1_NOME', 'Cliente Teste TIR')
        self.oHelper.SetButton('Cancelar')
        self.oHelper.AssertTrue()

    def test_SA1_CT002(self):
        """CT002 — Alterar cliente existente"""
        self.oHelper.SearchBrowse('01TEST01', key='Filial+codigo')
        self.oHelper.SetButton('Alterar')
        self.oHelper.SetValue('Nome', 'Cliente Teste Atualizado')
        self.oHelper.SetButton('Salvar')
        self.oHelper.SetButton('Cancelar')
        self.oHelper.AssertTrue()

    def test_SA1_CT003(self):
        """CT003 — Validacao de campo obrigatorio"""
        self.oHelper.SetButton('Incluir')
        self.oHelper.SetBranch('01')
        self.oHelper.SetButton('Salvar')
        self.oHelper.CheckHelp(text_help='E1-S01', button='Fechar')
        self.oHelper.SetButton('Cancelar')
        self.oHelper.AssertFalse()

    def test_SA1_CT004(self):
        """CT004 — Excluir cliente"""
        self.oHelper.SearchBrowse('01TEST01', key='Filial+codigo')
        self.oHelper.SetButton('Outras Acoes', sub_item='Excluir')
        self.oHelper.SetButton('Confirmar')
        self.oHelper.AssertTrue()

    @classmethod
    def tearDownClass(inst):
        """Teardown: Close session"""
        inst.oHelper.TearDown()

if __name__ == '__main__':
    unittest.main()
