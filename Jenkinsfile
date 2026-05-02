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
  }

  stages {
    stage('Checkout') {
      steps {
        checkout scm
      }
    }

    stage('Go test + coverage') {
      steps {
        // GO_VERSION из environment в одинарных ''' не попадает в bash надёжно; пустой ${GO_VERSION} даёт grep -F «go» → ложное совпадение и старый Go в /usr/local/go.
        sh """#!/bin/bash
set -eux
GO_VER='${env.GO_VERSION ?: '1.25.0'}'
export PATH="/usr/local/go/bin:\${PATH}"

ARCH="\$(uname -m)"
case "\$ARCH" in
  aarch64|arm64) GOARCH=arm64 ;;
  x86_64) GOARCH=amd64 ;;
  *) echo "unsupported arch: \$ARCH"; exit 1 ;;
esac

if command -v go >/dev/null 2>&1 && go version 2>/dev/null | grep -qF "go\${GO_VER}"; then
  echo "Go already at \${GO_VER}"
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
if ! command -v curl >/dev/null 2>&1 || ! command -v unzip >/dev/null 2>&1; then
  apt-get update -qq
  apt-get install -y -qq curl ca-certificates unzip
fi

ARCH="\$(uname -m)"
case "\$ARCH" in
  aarch64|arm64) ZIP_ARCH=aarch64 ;;
  x86_64) ZIP_ARCH=x64 ;;
  *) echo "unsupported arch: \$ARCH"; exit 1 ;;
esac

ZIP="sonar-scanner-cli-${env.SONAR_SCANNER_VERSION}-linux-\${ZIP_ARCH}.zip"
URL="https://binaries.sonarsource.com/Distribution/sonar-scanner-cli/\${ZIP}"
curl -fsSL "\$URL" -o "/tmp/\${ZIP}"
rm -rf /tmp/sonar-scanner-extract
mkdir -p /tmp/sonar-scanner-extract
unzip -q -o "/tmp/\${ZIP}" -d /tmp/sonar-scanner-extract
SCANNER_HOME="\$(find /tmp/sonar-scanner-extract -maxdepth 1 -type d -name 'sonar-scanner-*' | head -1)"
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
