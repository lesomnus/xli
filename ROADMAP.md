# xli Stable Release Roadmap

> `xli` 를 stable (`v1.0`) 로 릴리즈하기 위한 진행 계획.
> 미들웨어 + 중첩 context + strict flag/arg positioning 이라는 설계 정체성을 유지하면서,
> (1) 반쯤 만든 기능 마무리, (2) CLI 프레임워크 최소 기능 충족, (3) 공개 API 동결(freeze) 을 목표로 한다.

이 문서는 **살아있는 문서**다. 작업이 진행되면 맨 아래 [진행 현황](#진행-현황-progress) 의 체크박스가 갱신된다.

---

## 1. 설계 정체성 (변경하지 않는 것)

아래는 "버그"가 아니라 의도된 설계다. 리팩터링/수정 시 깨뜨리지 않는다.

- **미들웨어 체인**: `Command.Handler` 가 `next(ctx)` 를 직접 호출해야 자식으로 내려간다. `Run` 은 자식 핸들러 실행을 *보장하지 않는다*.
- **중첩 context**: 서브커맨드마다 context 가 중첩된다 (`frm`, `mode`, `tab`, `xli` 캐리어).
- **strict positioning**: 한 커맨드의 flag/arg 는 부모/자식에 나타날 수 없고, flag 는 반드시 arg 보다 앞에 와야 한다.
- **단일 실행 트리**: `Command` 트리는 한 번 실행되는 것을 전제로 한다 (root 는 보통 프로세스 싱글톤). 동시/재실행은 비지원 — 이건 *문서화*로 해결한다.

---

## 2. 현재 상태 스냅샷 (2026-06-21 기준)

- `go build ./...` : ✅ pass
- `go test ./...` : ✅ pass
- 커버리지: core **60.9%**, arg **38.3%**, flg **41.5%**, lex **89.2%**, `frm`/`mode`/`tab`/`internal/x`/`xmd` **0% 또는 없음**
- `go vet` : `countdown.go:42` unreachable code 1건

분석 방법: 7개 서브시스템 병렬 심층 분석 → 발견 버그 적대적 검증 → 프레임워크 완성도 평가 (26 에이전트).

---

## 3. 다운스트림 호환성 매트릭스 (깨면 안 되는 API)

`lesomnus/{arrakis, flob, clade, tegra-exporter}` 에서 실제로 사용 중인 심볼. 변경 시 이 목록을 기준으로 breaking 여부를 판정한다.

| 패키지 | 사용 중인 심볼 |
|---|---|
| `xli` | `Command`, `Next`, `OnRun`, `Chain`, `RequireSubcommand`, `OnRunPass`, `HandlerFunc`, `Handler`, `Commands`, `OnF` |
| `arg` | `MustGet`, `Args`, `String`, `Get`, `RestStrings`, `Mono` |
| `flg` | `String`, `VisitP`, `Flags`, `Switch`, `MustGet`, `Find`, `Base`, `Get` |
| `mode` | `Run`, `Pass`, `Mode` |
| `frm` | `HasSeq`, `From` |

**전혀 안 쓰는 것 → 자유롭게 수정/이름변경 가능**: `OnTap`/`OnTapPass`, tab/completion API 전체, `xli.S`/`xli.D`/`xli.Stringer`, `lex.*` 직접 사용, `Command.Root()`.

### 기본값(default) 의미론 — 확정 계약 (Phase 3 에서 구현, breaking 허용)

현재 `flg.Base.Value *T` 한 필드가 "생성 시 기본값" 과 "파싱된 사용자 값" 두 의미를 겸한다. `Get()` 이 `Value != nil` 로 `ok` 를 판정하므로, 기본값을 지정한 순간 "사용자가 안 줘도 `ok=true`" 가 되어 의미가 어긋난다 (precedence/"명시 여부" 로직이 조용히 틀어짐). 계약이 애초에 불명확했으므로 **breaking 을 허용하고** 역할을 분리한다.

**확정 계약**:
- `Default *T` — flag/arg 의 **기본값. 프레임워크가 절대 쓰지 않는(read-only) 값.** `nil` = 기본값 없음. (프레임워크 사용자가 지정)
- `Value *T` — **파싱된 사용자 값. 프레임워크가 파싱 때만 씀.** `nil`/`count==0` = 사용자가 안 줌.
- `Get` / `VisitP` / `Find` → `ok = (count > 0)`. 안 줬으면 `(zero, false)`; `Default` 는 보지 않는다.
- `MustGet` / `MustFind` → `Value(준 값) → Default → (둘 다 없으면) panic`. ("기본값 있으면 `MustGet`" 모델)
- `Handle` 은 `Value` 에만 쓴다 → 기존 "기본값 변수 덮어쓰기" footgun 자동 해소.
- help 의 기본값 표시는 `flg.Base.Default()` 메서드(현재 dead, 이름이 값 접근자와 혼동됨) 를 **제거**하고 `flg.Info.Default`(문자열) 로 옮긴다.
- `arg` 패키지에도 동일 원리 적용 (arg 는 default 주입 관용구가 드물어 우선순위는 낮음).

**다운스트림 영향** (교차 확인 완료):
- `port`/`kind` (arrakis, `MustGet`): ✅ 안전 — `MustGet` 이 `Default` 로 폴백.
- `graph` (clade, `Get`) / `config` (tegra, `Find`): ✅ 영향 없음 — 애초에 기본값 없음.
- **`diff` (arrakis `diff.go`)**: ❌ 깨짐 — `Value:&t` 로 스위치 값을 *코드 주입* 후 `VisitP` 로 읽음. 새 계약에선 "코드 주입 ≠ 사용자 입력" 이라 `VisitP` 가 false. → 마이그레이션: 주입 대신 파싱 경유로 바꾸거나 effective-value 접근자 사용.
- 전 다운스트림 `&flg.X{Value: ...}` → `&flg.X{Default: ...}` 기계적 변경 필요.

---

## 4. 확정 버그 목록 (adversarially verified)

| # | 위치 | 심각도 | 내용 | breaking? |
|---|---|---|---|---|
| B1 | `command.go:80` `Root()` | high | 항상 `nil` 반환 (루프가 `p != nil` 까지 돌아 끝에 nil). 다운스트림 미사용 → 순수 수정 | 아니오 |
| B2 | `help.go.tpl:18` | medium | Usage 에서 arg 사이 공백 없음 → `<SRC><DST>` | 아니오 |
| B3 | `lex/token.go:72` `indexes()` | high | all-dash flag (`-`, `--`) 에서 panic → `Name()/Arg()/WithArg()` 깨짐, `command.go:263` 경유 도달 가능 | 아니오 |
| B4 | `lex/token.go:149` `Spread()` | medium | 단일 short flag 에서 앞 dash 가 사라짐 (`-b`→`b`); degenerate flag panic | 아니오 |
| B5 | `arg/rest.go:57` | high | `Rest.Info()` 에 `Handle` 누락 → `RestStrings` 등 Rest 핸들러가 절대 실행 안 됨 (다운스트림 `arg.RestStrings` 영향) | 수정=개선 |
| B6 | `frame.go:148` | high | switch 판정에 `*flg.Switch` 구체 타입 단언 → 커스텀 no-value flag 불가. 인터페이스로 교체(가산적) | 아니오 |
| B7 | `string.go:11` `xli.S` | low | `Stringer` 인터페이스 시그니처 불일치로 `S` 사용 불가. 미사용 → 순수 수정 | 아니오 |
| B8 | `command.go:273` 외 | high | arg-value completion 이 죽어있음 (`TODO_Completion` 아무도 안 채움); short-flag value completion 오작동; 필수 arg 누락 시 flag-value completion 가림 | 아니오(미사용) |
| B9 | `handler.go:74` | low | `OnTap`/`OnTapPass` 가 `mode.Tab` 에 묶여있는데 이름이 "Tap" 오타. 다운스트림 미사용 → 이름 변경 + deprecated alias | 아니오 |
| B10 | `completion.go:32` | low | `NewCmdCompletion` zsh 핸들러가 2단계보다 얕게 mount 되면 nil-deref panic | 아니오 |

거부된(=의도된 설계) 항목: "Run 이 `Command` io/parent 필드를 mutate" (단일 실행 트리 전제이므로 버그 아님 → 문서화로 처리), `NormalizeCompletionArgs` 의 `buff` 인덱싱(계약 문서화로 처리).

---

## 5. 단계별 로드맵

각 단계는 독립적으로 머지 가능하며, 매 단계 후 **다운스트림 4개 레포를 `replace` 로 빌드/테스트**해 회귀를 검증한다.

### Phase 0 — 정확성 안정화 (non-breaking 버그 픽스 + 테스트) ✅ **구현 완료, 다운스트림 검증 중**
순수 버그 픽스와 누락 테스트. 공개 API 시그니처 변경 없음.
- [x] B1 `Root()` 픽스 + 테스트 (`tree_test.go`)
- [x] B2 help Usage 공백 픽스 + help 렌더링 테스트 (`help_test.go`; required `<X>`/optional `[X]`/variadic `[X...]` 검증)
- [x] B7 `xli.S` 시그니처 픽스 + compile-time 어서션 테스트 (`string_test.go`)
- [x] B5 `Rest.Info()` 에 `Handle` 추가 + 테스트 (`arg/rest_test.go`)
- [x] B3/B4 `lex` panic/dash-drop 하드닝 + 테스트 (`lex/token_test.go`)
- [x] B6 switch 판정을 optional `NoValue()` 인터페이스로 교체 (`flg.Switch` 계속 동작, 커스텀 switch 가능) + 테스트 (`flg/switch_test.go`)
- [x] B9 `OnTap`→`OnTab` 이름 변경, `OnTap`/`OnTapPass` 는 deprecated alias 유지 (xli/arg/flg) + 테스트
- [x] `countdown.go:42` unreachable 제거 + until-done/ctx-done 분기 테스트
- [x] frame.prepare 의 입력-도달 panic(`frame.go:248/276`) 을 `ErrNeedArgs` 로 전환 (Run 은 이미 error 반환 → non-breaking) + 테스트 (`frame_prepare_test.go`)
  - 참고: `frame.go:254`(파서가 받은 토큰보다 많이 소비했다고 보고) 는 *커스텀 Parser 구현 계약 위반*이라 panic 유지 (사용자 입력으로 도달 불가).

**결과**: `go build`/`go vet` 클린, 전체 테스트 통과. 커버리지 core 60.9→**64.0%**, arg 38.3→**46.8%**, flg 41.5→**44.7%**, lex 89.2→**89.9%**.

### Phase 1 — 라이브러리 위생 (robustness) ✅ **완료**
사용자 입력으로 도달 가능한 모든 panic 제거, 에러 보고 일관화.
- [x] 에러 sentinel 표준화: flag-after-arg 를 `ErrFlagAfterArg` + `FlagError` 로 (`errors.Is` 가능), "are must be set at the behind" 문법 오류 제거 ([frame.go](frame.go), [errors.go](errors.go)) + 테스트
- [x] dead 코드 제거: `mode.Resolve` 삭제 (+ `slices` import 제거)
- [x] mode 상수 전부 `Mode` 타입으로 통일 (freeze 전 hygiene; 다운스트림 연산자 우선순위 안전 확인)
- [x] `HasSeq` nil 가드 추가 (이름이 체인보다 길 때 panic 방지 — 다운스트림 `frm.HasSeq` 보호)
- [x] `frm`/`mode`/`tab` 테스트 추가 → **각 100% 커버리지**
- [x] 도달 불가 방어 panic(`frame.go:171/196`) 은 lexer 가 4개 토큰만 반환하므로 "사용자 도달 불가 불변식" 으로 주석 명시(변환 안 함 — DoD 충족)

**Phase 1 에서 제외(범위 재조정)**:
- `command.go:192/277` 의 panic 은 completion 엔진(`runCompletion`) 내부 → **Phase 2 에서 재작성과 함께 처리** (지금 단독 변환은 폐기성 작업).
- `arg.IsMany` 는 `arg.Arg` 인터페이스 멤버이고 Phase 3 usage 포맷(variadic 판정)에 쓸 수 있어 **유지** (제거 결정은 Phase 4 freeze).

**결과**: `go build`/`go vet`/`go test -race` 클린, 전체 통과. 다운스트림 재검증 — flob/clade/tegra **build+test PASS**, arrakis build PASS(테스트 실패는 동일한 xli-무관 `arks` 패키지).

### Phase 2 — Completion 마무리 (B8 본체)
tab completion API 를 완성하고 동결 가능한 형태로 정리.
- [ ] arg-value completion 을 arg 의 `Handle`(mode-gated) 경유로 라우팅, `TODO_Completion` 제거/개명
- [ ] short-flag value completion 픽스 (`GetByAlias`, `-x=` 처리)
- [ ] 필수 arg 누락이 flag-value completion 을 가리지 않도록 분리
- [ ] `tab.Tab` 인터페이스 확장(directive/grouping/error) 후 동결
- [ ] completion 통합 테스트 추가
- [ ] (post-freeze) bash/fish/powershell 셸 추가 — Tab 인터페이스 동결 이후

### Phase 3 — 프레임워크 기능 충족 (feature parity)
"진짜 CLI 프레임워크" 최소 기능. API 변경을 수반하므로 freeze 전에 끝낸다.
- [ ] 내장 `--version`/`-v` + `Command.Version` 필드
- [ ] 필수 flag 선언 + 검증
- [ ] **기본값 의미론 정리** (확정 계약, breaking 허용 — 위 [기본값 의미론](#기본값default-의미론--확정-계약-phase-3-에서-구현-breaking-허용) 절 참조): `Default`/`Value` 필드 분리, `Get`=`count>0`, `MustGet`=`Value→Default→panic`, `Default()` 메서드 제거 후 `flg.Info.Default` 로 help 표시, arrakis 마이그레이션
- [ ] flag 값 타입 추가: `float`, `time.Duration`, repeatable/`[]string`
- [ ] custom help template 주입 훅 (`PrintHelp` 의 TODO) + 템플릿 1회 파싱 캐시
- [ ] `Synop`(long description) 렌더링 또는 필드 제거 결정
- [ ] usage 자동 포맷 컨벤션 확정 (현재 `<req>`/`[opt]`/`[opt...]` 유지 여부; 사용자는 optional→`[ARG]` 만 명시)
- [ ] (nice-to-have) env-var 바인딩, enum/choice, 상호배타 그룹

### Phase 4 — API 동결 & 폴리시 → `v1.0`
- [ ] 공개 API 최종 점검 (mode 상수 타입 통일, 죽은 export 제거, 네이밍 일관성)
- [ ] 단일 실행 트리/핸들러가 `next()` 호출 책임/strict positioning 등 "의도된 날카로운 모서리" 문서화
- [ ] README 작성 (현재 2줄), 예제 보강, godoc
- [ ] 다운스트림 4개 레포 최종 회귀 통과
- [ ] `v1.0.0` 태그

---

## 6. "Stable" 의 정의 (Definition of Done)

1. 사용자 입력으로 도달 가능한 panic 0개 (모두 error 반환).
2. 확정 버그(섹션 4) 전부 해결 + 회귀 테스트.
3. 최소 기능: subcommand/help/version/required·optional/기본값/공통 값타입/완성된 zsh completion.
4. 전 패키지 의미 있는 테스트 커버리지 (특히 0% 패키지 해소).
5. 공개 API 동결 — `TODO_` export·오타·죽은 export 제거.
6. 다운스트림 4개 레포가 수정 없이(또는 문서화된 최소 마이그레이션으로) 빌드/테스트 통과.
7. README + godoc.

---

## 7. 진행 현황 (Progress)

- **2026-06-21**: 코드베이스 분석 완료, 다운스트림 사용 표면 조사 완료, 로드맵 수립.
- **2026-06-21**: **Phase 0 완료.** 확정 버그 B1~B7, B9 + countdown + frame.prepare panic→error 픽스 및 회귀 테스트 작성.
  - `go build`/`go vet`/`go test -race` 모두 클린. 신규 테스트 파일: `tree_test.go`, `help_test.go`, `string_test.go`, `frame_prepare_test.go`, `flg/switch_test.go` + 기존 테스트 확장.
  - **다운스트림 회귀 검증** (로컬 xli `replace` 후 빌드/테스트):
    - arrakis: build ✅ / xli 사용처(`cmd`) OK — 테스트 실패는 xli 미사용 `arks` 패키지의 기존 이슈
    - flob: build ✅ / test ✅
    - clade: build ✅ / test ✅ (`clade/cmd` 포함)
    - tegra-exporter: build ✅ / test ✅
  - → **breaking change 없음** 확인.

- **2026-06-21**: **Phase 1 완료.** 에러 sentinel 표준화(`ErrFlagAfterArg`), `mode.Resolve` 제거 + mode 상수 타입 통일, `HasSeq` nil 가드, `frm`/`mode`/`tab` 테스트(각 100%). 도달 불가 방어 panic 은 불변식 주석으로 명시. `go test -race` 클린, 다운스트림 회귀 통과.

### 다음 작업: Phase 2 (Completion 마무리)
arg-value completion 활성화(`TODO_Completion` 제거 → `Handle` 경유), short-flag value completion 픽스, 필수 arg 가 flag-value completion 을 가리는 문제 분리, `tab.Tab` 인터페이스 확장·동결, `runCompletion` 의 panic(`command.go:192/277`)·error-swallow 정리, completion 통합 테스트. (B8/B10)

> 미해결 확정 버그 중 Phase 0 에서 제외한 것: **B8(completion 본체)** → Phase 2, **B10(`NewCmdCompletion` nil-deref)** → Phase 2 와 함께. 기본값 의미론(landmine) → Phase 3.
