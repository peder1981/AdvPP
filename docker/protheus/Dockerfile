# Ambiente Protheus para compilação e injeção de bytecode
FROM debian:12-slim

ENV DEBIAN_FRONTEND=noninteractive
ENV LANG=C
ENV LC_ALL=C

# Dependências
RUN apt-get update && apt-get install -y \
    bash \
    curl \
    tar \
    gzip \
    procps \
    iputils-ping \
    net-tools \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Usuário totvs
RUN groupadd -g 1000 totvs && \
    useradd -m -u 1000 -g 1000 -s /bin/bash totvs

# Variáveis
ENV TOTVS_VERSION=12.1.2510
ENV TOTVS_PATH=/totvs/protheus${TOTVS_VERSION}
ENV APPSERVER_PATH=${TOTVS_PATH}/bin/appserver
ENV APO_PATH=${TOTVS_PATH}/apo
ENV DATA_PATH=${TOTVS_PATH}/protheus_data

# Copiar arquivos (estrutura: protheus/bin/, protheus/apo/)
COPY protheus/bin/ ${TOTVS_PATH}/bin/
COPY protheus/apo/ ${TOTVS_PATH}/apo/
COPY docker/scripts/entrypoint.sh /entrypoint.sh

# Extrair appserver
RUN cd ${TOTVS_PATH}/bin && \
    tar xzf appserver_linux/appsrvlinux.tar.gz && \
    mv appserver_linux/* . && \
    rm -rf appserver_linux && \
    chmod +x appsrvlinux

# Permissões
RUN mkdir -p ${DATA_PATH} ${APO_PATH} && \
    chown -R totvs:tots ${TOTVS_PATH} 2>/dev/null || chown -R totvs:totvs ${TOTVS_PATH} && \
    chmod -R 755 ${TOTVS_PATH} && \
    chmod 777 ${DATA_PATH} && \
    chmod +x /entrypoint.sh

# Porta
EXPOSE 3999 8090 32033

WORKDIR ${APPSERVER_PATH}

ENTRYPOINT ["/entrypoint.sh"]
CMD ["sleep", "infinity"]
