#!/bin/bash
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

if [ ! -f "deploy.conf" ]; then
    echo -e "${RED}Ошибка: deploy.conf не найден${NC}"
    exit 1
fi

source deploy.conf

if [ -z "$DEPLOY_HOST" ] || [ -z "$DEPLOY_USER" ] || [ -z "$DEPLOY_PATH" ]; then
    echo -e "${RED}Ошибка: Не заполнены обязательные переменные в deploy.conf${NC}"
    exit 1
fi

SSH_CMD="ssh -p ${DEPLOY_PORT:-22} ${DEPLOY_USER}@${DEPLOY_HOST}"

echo -e "${YELLOW}=== Деплой Banki на ${DEPLOY_HOST} ===${NC}"
echo -e "Репозиторий: ${GIT_REPO}"
echo -e "Ветка: ${GIT_BRANCH}"
echo -e "Путь: ${DEPLOY_PATH}"
echo ""

# 1. Проверяем SSH
echo -e "${GREEN}[1/5] Проверяю SSH подключение...${NC}"
if ! $SSH_CMD "echo 'SSH OK'" 2>/dev/null; then
    echo -e "${RED}Не удалось подключиться по SSH${NC}"
    exit 1
fi

# 2. Клонируем/проверяем репозиторий
echo -e "${GREEN}[2/5] Проверяю репозиторий на сервере...${NC}"
$SSH_CMD bash -s << EOF
    set -e
    mkdir -p \$(dirname ${DEPLOY_PATH})

    if [ ! -d ${DEPLOY_PATH}/src ]; then
        echo 'Репозиторий не найден, клонирую...'
        git clone ${GIT_REPO} ${DEPLOY_PATH}/src
    elif [ ! -d ${DEPLOY_PATH}/src/.git ]; then
        echo 'Директория не является git репозиторием, переклонирую...'
        rm -rf ${DEPLOY_PATH}/src
        git clone ${GIT_REPO} ${DEPLOY_PATH}/src
    else
        echo 'Репозиторий найден'
    fi
EOF

# 3. Обновляем код
echo -e "${GREEN}[3/5] Обновляю код из ${GIT_BRANCH}...${NC}"
$SSH_CMD bash -s << EOF
    set -e
    cd ${DEPLOY_PATH}/src
    git fetch origin
    git checkout ${GIT_BRANCH}
    git reset --hard origin/${GIT_BRANCH}
    echo 'Код обновлён до последней версии'
EOF

# 4. Проверяем .env
echo -e "${GREEN}[4/5] Проверяю .env...${NC}"
$SSH_CMD bash -s << EOF
    set -e
    cd ${DEPLOY_PATH}
    if [ ! -f .env ]; then
        echo ''
        echo -e 'ВНИМАНИЕ: .env не найден!'
        echo ''
        echo 'Создай файл на сервере:'
        echo '  ssh ${DEPLOY_USER}@${DEPLOY_HOST}'
        echo '  cd ${DEPLOY_PATH}'
        echo '  nano .env'
        echo ''
        echo 'Обязательные переменные:'
        echo '  BANKI_SESSION_SECRET=<openssl rand -hex 32>'
        echo ''
        echo 'Опционально:'
        echo '  BANKI_DEFAULT_USER=admin'
        echo '  BANKI_DEFAULT_PASS=admin'
        exit 1
    else
        echo '.env найден'
    fi
EOF

# 5. Собираем образ и запускаем
echo -e "${GREEN}[5/5] Собираю образ и запускаю...${NC}"
$SSH_CMD bash -s << EOF
    set -e
    cd ${DEPLOY_PATH}/src
    docker build -t ${IMAGE_NAME}:latest .
    cp docker-compose.yml ${DEPLOY_PATH}/docker-compose.yml
    cd ${DEPLOY_PATH}
    docker compose up -d

    echo ''
    echo '=== Статус контейнеров ==='
    docker compose ps
EOF

echo ""
echo -e "${GREEN}=== Деплой завершён успешно! ===${NC}"
echo -e "Приложение: ${YELLOW}http://${DEPLOY_HOST}:8080${NC}"
echo -e "Логи: ${YELLOW}make deploy-logs${NC}"
