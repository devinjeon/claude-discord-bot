# claude-discord-bot

Discord를 통해 Claude Code tmux 세션을 원격으로 모니터링하고 제어하는 시스템.

## 구성 요소

이 프로젝트는 두 개의 서비스로 구성됩니다:

| 서비스 | tmux 세션 | 역할 |
|--------|-----------|------|
| claude-channel | `claude-channel` | Claude Code를 Discord 채널 플러그인과 함께 실행 |
| claude-bot | `claude-bot` | claude-channel 세션을 모니터링하고 Discord를 통해 제어 |

claude-channel이 실제 Claude Code 에이전트이고, claude-bot은 이 에이전트의 tmux 세션을 감시하며 사용자 개입이 필요한 상황(선택지, 텍스트 입력)을 Discord로 중계합니다.

## 기능

### Discord 명령어
- `/claude-now` - tmux 세션의 현재 터미널 출력 확인
- `/claude-screenshot` - 시스템 스크린샷 촬영 후 전송
- `/claude-restart` - Claude tmux 세션 재시작 (LaunchAgent가 자동 복구)
- `/claude-usage` - Claude Code 사용량 확인

### 자동 기능
- 선택지 감지 - tmux에 선택 프롬프트가 나타나면 Discord에 이모지 버튼으로 알림
- 텍스트 입력 중계 - Claude가 텍스트 입력을 요청하면 Discord 메시지로 입력 가능
- 재시작 알림 - Claude 세션이 시작되면 Discord에 자동 알림 (SessionStart hook)

## 동작 흐름

```
┌─────────────┐      ┌─────────────┐      ┌─────────────────┐
│   Discord    │◄────►│  claude-bot  │─────►│  claude-channel  │
│   (사용자)   │      │  (Go 봇)    │ tmux │  (Claude Code)   │
└─────────────┘      └─────────────┘      └─────────────────┘
       ▲                                          │
       └──────────────────────────────────────────┘
                   Discord 플러그인으로 직접 대화
```

1. `run-channel.sh`가 tmux 세션에서 Claude Code + Discord 플러그인을 실행
2. `run-bot.sh`가 별도 tmux 세션에서 Go 봇을 실행
3. 봇이 5초 간격으로 claude-channel 세션을 폴링하여 선택 프롬프트 감지
4. 감지된 선택지를 Discord 채널에 이모지 반응과 함께 전송
5. 사용자가 이모지를 클릭하면 해당 선택을 tmux에 키 입력으로 전달
6. 두 프로세스 모두 macOS LaunchAgent로 관리되어 자동 시작/재시작

## 프로젝트 구조

```
.
├── cmd/claude-bot/                # Go 봇 엔트리포인트
│   └── main.go
├── internal/
│   ├── bot/                       # Discord 봇 핵심 로직
│   │   ├── bot.go                 # 초기화, 핸들러 등록, 실행
│   │   ├── commands.go            # 슬래시/메시지 커맨드 핸들러
│   │   ├── polling.go             # tmux 선택지 폴링 + 텍스트 입력 감지
│   │   └── state.go               # 상호작용 상태 관리
│   ├── tmux/                      # tmux 조작 유틸리티
│   │   └── tmux.go
│   └── imaging/                   # 스크린샷 + 이미지 처리
│       └── screenshot.go
├── deploy/
│   ├── bot/                       # claude-bot LaunchAgent
│   │   ├── com.devin.claude-bot.plist
│   │   └── run-bot.sh
│   ├── claude-channel/            # claude-channel LaunchAgent
│   │   ├── com.devin.claude-channel.plist
│   │   ├── run-channel.sh
│   │   └── discord-restart-notify.sh
│   ├── install.sh
│   └── uninstall.sh
├── .github/workflows/
│   └── release.yml                # GoReleaser GitHub Actions
├── .goreleaser.yml
├── Makefile
├── go.mod
└── go.sum
```

## 설정

### 환경 변수

| 변수 | 설명 | 기본값 |
|------|------|--------|
| `DISCORD_BOT_TOKEN` | Discord 봇 토큰 | `~/.claude/channels/discord/.env`에서 로드 |
| `DISCORD_CHANNEL_ID` | 대상 채널 ID | `1485304276107395164` |
| `DISCORD_GUILD_ID` | 서버(길드) ID | `1485304275591626802` |

토큰은 환경 변수 또는 `~/.claude/channels/discord/.env` 파일에서 `DISCORD_BOT_TOKEN=...` 형식으로 설정할 수 있습니다.

## 빌드 및 실행

```bash
# 빌드
make build

# 직접 실행
make run

# 또는
go build -o claude-bot ./cmd/claude-bot
DISCORD_BOT_TOKEN=your-token ./claude-bot
```

## 배포

```bash
# 빌드 + LaunchAgent 설치 + hook 설정
make install

# 제거 (서비스 중지 + plist 제거 + hook 제거)
make uninstall
```

install.sh가 수행하는 작업:
1. Go 바이너리 빌드
2. 기존 서비스 중지
3. LaunchAgent plist 심링크 (`~/Library/LaunchAgents/`)
4. Claude Code SessionStart hook 설치 (재시작 알림)
5. 서비스 로드

## 릴리스

태그를 푸시하면 GitHub Actions에서 GoReleaser가 자동으로 바이너리를 빌드하고 릴리스합니다:

```bash
git tag v0.1.0
git push origin v0.1.0
```
