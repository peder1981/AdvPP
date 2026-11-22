-- Fixture do adapter FWAdapterBaseV2 (plano PO-UI onda 1, Task 7).
-- Tabela de negócio NNR (armazéns) + entradas SX3 para TamSx3/AddMapFields.

CREATE TABLE IF NOT EXISTS NNR (
	R_E_C_N_O_   INTEGER PRIMARY KEY AUTOINCREMENT,
	D_E_L_E_T_   TEXT DEFAULT ' ',
	NNR_FILIAL   TEXT,
	NNR_CODIGO   TEXT,
	NNR_DESCRI   TEXT,
	NNR_ESTOQUE  REAL
);

CREATE TABLE IF NOT EXISTS SX3 (
	X3_CAMPO    TEXT,
	X3_TITULO   TEXT,
	X3_TIPO     TEXT,
	X3_TAMANHO  TEXT,
	X3_DECIMAL  TEXT,
	X3_ARQUIVO  TEXT,
	X3_ORDEM    TEXT
);

INSERT INTO SX3 (X3_CAMPO, X3_TITULO, X3_TIPO, X3_TAMANHO, X3_DECIMAL, X3_ARQUIVO, X3_ORDEM) VALUES
('NNR_CODIGO',  'Codigo',   'C', '10', '0', 'NNR', '1'),
('NNR_DESCRI',  'Descricao', 'C', '40', '0', 'NNR', '2'),
('NNR_ESTOQUE', 'Estoque',  'N', '12', '2', 'NNR', '3'),
('NNR_FILIAL',  'Filial',   'C', '6',  '0', 'NNR', '4');
