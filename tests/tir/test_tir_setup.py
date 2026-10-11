"""
Teste de configuracao do TIR
"""
import os
import sys

# Adicionar ChromeDriver ao PATH
os.environ['PATH'] += ':/usr/bin'

from tir import Webapp
import unittest

class TIRSetupTest(unittest.TestCase):
    """Teste de conexao com Protheus Webapp"""

    @classmethod
    def setUpClass(inst):
        """Setup: Inicializar Webapp"""
        inst.oHelper = Webapp(config_path='config.json')
        inst.oHelper.Setup('MATA', '20260929', 'T1', '01')

    def test_001_conexao(self):
        """CT001 — Verificar conexao com Webapp"""
        # Verificar se a pagina carrego
        self.assertTrue(True)  # Se chegou aqui, a conexao funcionou

    def test_002_acessar_sa1(self):
        """CT002 — Acessar rotina SA1"""
        self.oHelper.Program('SA1')
        # Verificar se a rotina foi aberta
        self.oHelper.WaitShow('Clientes')

    @classmethod
    def tearDownClass(inst):
        """Teardown: Fechar sessao"""
        inst.oHelper.TearDown()

if __name__ == '__main__':
    unittest.main()
