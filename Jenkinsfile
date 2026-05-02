// Jenkins: Go-тесты + SonarScanner без вложенного «docker run -v $WORKSPACE».
// Иначе Docker-демон на хосте не видит файлы из volume Jenkins → пустой /workspace → нет go.mod.
//
// Здесь: ставим Go и sonar-scanner-cli внутри контейнера Jenkins (curl + tar/unzip).

pipeline {
  agent any

  environment {
    SONAR_HOST_URL = 'http://host.docker.internal:9000'
    // Совпадает с директивой `go` в go.mod; полный tarball + GOTOOLCHAIN=local — иначе `covdata` при GOTOOLCHAIN=auto.
    GO_VERSION = '1.25.0'
    SONAR_SCANNER_VERSION = '8.0.1.6346'
    // Для сенсора JS/TS/CSS Sonar нужен Node.js в PATH (static/js, static/css в репозитории).
    NODE_JS_VERSION = '20.18.1'
  }

  stages {
    stage('Checkout') {
      steps {
        checkout scm
      }
    }

    stage('Go test + coverage') {
      steps {
        // GOTOOLCHAIN=local до любого вызова go: иначе go version под auto скачает toolchain и покажет 1.25, а после local останется старый GOROOT 1.23.
        // Проверяем только /usr/local/go/bin/go — не «go» из PATH с другим поведением.
        sh """#!/bin/bash
set -eux
GO_VER='${env.GO_VERSION ?: '1.25.0'}'
export GOTOOLCHAIN=local
export PATH="/usr/local/go/bin:\${PATH}"

ARCH="\$(uname -m)"
case "\$ARCH" in
  aarch64|arm64) GOARCH=arm64 ;;
  x86_64) GOARCH=amd64 ;;
  *) echo "unsupported arch: \$ARCH"; exit 1 ;;
esac

if [ -x /usr/local/go/bin/go ] && /usr/local/go/bin/go version 2>/dev/null | grep -qF "go\${GO_VER}"; then
  echo "Go already at \${GO_VER} under /usr/local/go"
else
  curl -fsSL "https://go.dev/dl/go\${GO_VER}.linux-\${GOARCH}.tar.gz" -o /tmp/go.tgz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tgz
fi

export GOROOT=/usr/local/go
export GOTOOLCHAIN=local

go version
cd "\${WORKSPACE}"
go test ./... -coverprofile=coverage.out -covermode=atomic
"""
      }
    }

    stage('SonarQube analysis') {
      environment {
        SONAR_TOKEN = credentials('sonarqube-token')
      }
      steps {
        sh """#!/bin/bash
set -eux
if ! command -v curl >/dev/null 2>&1 || ! command -v unzip >/dev/null 2>&1 || ! command -v xz >/dev/null 2>&1; then
  apt-get update -qq
  apt-get install -y -qq curl ca-certificates unzip xz-utils
fi

ARCH="\$(uname -m)"
case "\$ARCH" in
  aarch64|arm64) ZIP_ARCH=aarch64; NODE_DIST_ARCH=arm64 ;;
  x86_64) ZIP_ARCH=x64; NODE_DIST_ARCH=x64 ;;
  *) echo "unsupported arch: \$ARCH"; exit 1 ;;
esac

NODE_VER='${env.NODE_JS_VERSION ?: '20.18.1'}'
NODE_BASE="node-v\${NODE_VER}-linux-\${NODE_DIST_ARCH}"
NODE_ROOT="/usr/local/\${NODE_BASE}"
if [ ! -x "\${NODE_ROOT}/bin/node" ]; then
  curl -fsSL "https://nodejs.org/dist/v\${NODE_VER}/\${NODE_BASE}.tar.xz" -o /tmp/node.txz
  tar -C /usr/local -xJf /tmp/node.txz
fi
export PATH="\${NODE_ROOT}/bin:\${PATH}"
node -v

ZIP="sonar-scanner-cli-${env.SONAR_SCANNER_VERSION}-linux-\${ZIP_ARCH}.zip"
URL="https://binaries.sonarsource.com/Distribution/sonar-scanner-cli/\${ZIP}"
curl -fsSL "\$URL" -o "/tmp/\${ZIP}"
# Родитель нельзя называть sonar-scanner-* — find совпадёт с ним раньше, чем с каталогом из zip.
SCANNER_ROOT=/tmp/ss-unpack
rm -rf "\${SCANNER_ROOT}"
mkdir -p "\${SCANNER_ROOT}"
unzip -q -o "/tmp/\${ZIP}" -d "\${SCANNER_ROOT}"
SCANNER_HOME="\$(find "\${SCANNER_ROOT}" -maxdepth 1 -mindepth 1 -type d -name 'sonar-scanner-*' | head -1)"
test -x "\${SCANNER_HOME}/bin/sonar-scanner"

cd "\${WORKSPACE}"
"\${SCANNER_HOME}/bin/sonar-scanner" \\
  -Dsonar.host.url="${env.SONAR_HOST_URL}" \\
  -Dsonar.token="\${SONAR_TOKEN}"
"""
      }
    }
  }
}
