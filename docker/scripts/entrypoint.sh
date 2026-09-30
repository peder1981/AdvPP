#!/bin/bash
# Entry point para container Protheus

set -e

APPSERVER_PATH=/totvs/protheus12.1.2510/bin
APO_PATH=/totvs/protheus12.1.2510/apo
DATA_PATH=/totvs/protheus12.1.2510/protheus_data

# Criar diretórios
mkdir -p ${DATA_PATH} ${APO_PATH}

# Copiar INI para o local correto
cat > ${APPSERVER_PATH}/appserver.ini << 'INIEOF'
[environment]
SourcePath=/totvs/protheus12.1.2510/apo
RootPath=/totvs/protheus12.1.2510/protheus_data
StartPath=/system/
RpoCustom=/totvs/protheus12.1.2510/apo/custom.rpo
x2_path=
RpoDb=top
RpoLanguage=multi
RpoVersion=120
LocalFiles=CTREE
Trace=0
localdbextension=.dtc
StartSysInDB=1
consolelog=1
topmemomega=50

[Drivers]
Active=TCP

[TCP]
TYPE=TCPIP
Port=3999

[Service]
Name=protheus1212510
DisplayName=TOTVS Protheus 12.1.2510

[LICENSECLIENT]
server=license-server
port=5555

[TM1]
Company=99
Branch=01
Rpo=custom.rpo
INIEOF

# Copiar tttm120.rpo para custom.rpo como padrão
if [ ! -f ${APO_PATH}/custom.rpo ]; then
    cp ${APO_PATH}/tttm120.rpo ${APO_PATH}/custom.rpo 2>/dev/null || true
fi

# Mudar para o diretório do appserver
cd ${APPSERVER_PATH}

# Se houver argumento, executar
if [ "$1" != "sleep" ]; then
    export LD_LIBRARY_PATH=.
    exec "$@"
fi

# Modo padrão
exec "$@"
