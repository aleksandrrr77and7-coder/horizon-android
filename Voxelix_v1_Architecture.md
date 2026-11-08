# Voxelix v1.0 — Технический план оптимизации рендеринга Horizon на Android

**Целевое устройство:** OPPO CPH2727, Snapdragon 6s 4G, Adreno 610, Android 16, 8 GB RAM
**Версия Horizon:** 26.3 (Android fork)
**Статус:** Технический план, исходный код не изменялся
**Язык:** русский

---

## 1. Введение

Voxelix — набор независимых оптимизаций клиентского рендеринг-пути Horizon для Android-устройств среднего уровня (Adreno 610). Цель — снизить нагрузку на CPU и GPU в конвейере отрисовки чанков/секций, сохранив внешний вид идентичным ванили 26.3.

Все оптимизации модульны, каждая имеет отдельный переключатель (ключ запуска / настройку), можно отключать по отдельности для A/B и отката. Касаются только `crates/horizon-client-render` и смежных слоёв Android-рантайма. Игровая логика (`horizon-world`, `horizon-client` bin) не трогается.

---

## 2. Целевое устройство и его características

| Компонент | Значение | Значение для рендеринга |
|---|---|---|
| SoC | Snapdragon 6s 4G (предположительно Snapdragon 6 Gen 4 / 6s 4G) | 8 ядер, интегрированный GPU, unified memory |
| GPU | Adreno 610 | Tile-based deferred renderer (TBDR), Vulkan 1.1+, GLES 3.2 |
| RAM | 8 GB (shared CPU+GPU) | Нет дикретной видеопамяти, буферы борются за системную память |
| ОС | Android 16 | wgpu Vulkan бэкенд предпочтителен, OpenGL ES как fallback |
| Дисплей | Неизвестно, обычно 1080p или выше | FrameScale / scaled presentation на Android уже есть |

Adreno 610 — интегрированный мобильный GPU архитектуры Adreno 600. Поддержка Vulkan features (indirect draw, shader storage, early depth test) требует проверки через `wgpu::Adapter::features()` на реальном устройстве; на уровне исходников она не выражена явно для Adreno.

---

## 3. Подтверждённые факты из исходников

Ниже — только то, что直接 подтверждено чтением кода. Ссылки даются на файлы и строки на момент проверки.

### 3.1 multi_draw выключен на Android жестко

**Файл:** `crates/horizon-client-render/src/gfx.rs`, строка 3597:

```rust
let multi_draw = !cfg!(target_os = "android")
    && device
        .features()
        .contains(wgpu::Features::INDIRECT_FIRST_INSTANCE)
    && adapter
        .get_downlevel_capabilities()
        .flags
        .contains(wgpu::DownlevelFlags::INDIRECT_EXECUTION);
```

На Android `multi_draw` всегда `false`, даже если аппарат поддерживает `INDIRECT_FIRST_INSTANCE` / `INDIRECT_EXECUTION`. Вызов рендеринга секций (`draw_sections`, стр. 4862) при `multi_draw == false` идёт циклом `draw_indexed` по каждой секции (стр. 4889-4895), без `multi_draw_indexed_indirect`.

### 3.2 GPU occlusion culling выключен на Android жестко

**Файл:** `crates/horizon-client-render/src/gfx/cull.rs`, строки 190-193:

```rust
pub fn supported(adapter: &wgpu::Adapter, device: &wgpu::Device) -> Option<DepthTest> {
    if cfg!(target_os = "android") {
        return None;
    }
```

Далее в `gfx.rs` (строки 3624-3627) двойная защита:

```rust
let cull = cull::Cull::supported(&adapter, &device)
    .filter(|_| !cfg!(target_os = "android"))
    .filter(|_| !crate::android_runtime::compatibility())
    .filter(|_| gpu_culling_requested(&adapter_info, cull_env.as_deref().ok()));
```

`crate::android_runtime::compatibility()` (файл `android_runtime.rs`, строки 177-179):

```rust
pub fn compatibility() -> bool {
    cfg!(target_os = "android") && !GPU_BATCHING.load(Ordering::Relaxed)
}
```

По умолчанию `GPU_BATCHING == false` (стр. 124), поэтому на Android `compatibility() == true` и GPU occlusion недоступен даже если бы не был отключён `cfg`.

### 3.3 CPU occlusion culling (SectionOcclusionGraph) работает на всех платформах

**Файл:** `crates/horizon-client-render/src/gfx/occlusion.rs`
Реализует `SectionOcclusionGraph` — поиск в ширину от секции камеры через открытые граней (`VisGraph`), العامل вне потока рендеринга через `mpsc` каналы. Кадр использует последний готовый граф и не ждёт следующего. Вызывается из `draw_frame` (`start_occlusion`, стр. 6360-6362).

### 3.4 Frustum culling и видимость секций

**Файл:** `crates/horizon-client-render/src/gfx.rs`, `fn draw_frame` (строки 6323-6387): построение усекающей пирамиды, `self.sections.visible()` с проверкой `reachable.contains(k)` (occlusion graph) и дистанции (`fog_culled_radius`, стр. 6355). LOD-тайлы visible отдельно (стр. 6391-6400).

### 3.5 Переиспользование SectionMesh через MeshPool

**Файл:** `crates/horizon-client-render/src/gfx.rs` — `sections: resident::Resident<SectionGpu>` (стр. 831), `release_section` / `release_mesh` (стр. 4906-4921) возвращают страницы арены и отсортированные индексы. `arena::Arena` (`gfx/arena.rs`) хранит вершины секций в страницах; `compact_arena` (стр. 4928) дефрагментирует страницы, возвращая пустые.

### 3.6 Фоновые workers для построения мешей

**Файл:** `crates/horizon-client-render/src/streaming.rs`
Пул воркеров генерирует, освещает и мешит колонки чанков ближайшие zuerst. Игровой поток передаёт неизменяемые `Arc` снимки; результаты носят порядковый номер, устарелая работа отбрасывается. `IDLE_RELEASE` 5 секунд (стр. 27).

### 3.7 Android-специфичные модули рендеринга

- `gfx/frame_scale.rs` — масштабирование кадра через существующий post-process копирующий проход (`#[cfg(target_os = "android")]`, стр. 213-214 в `gfx.rs`).
- `gfx/opaque_output.rs` — затирка альфа-канала для Android-композитора (`#[cfg(target_os = "android")]`, стр. 233-234).
- `graphics_startup.rs` — маркер прерванного запуска Vulkan на Android.

### 3.8 Upload budget с мобильными лимитами

**Файл:** `crates/horizon-client-render/src/gfx/upload_budget.rs`
Мобильный бюджет загрузки: 2–4 MiB на кадр, 1–3 мс времени, ужесточается при `frame_dt > 22 мс` (стр. 45-47). `gpu_batching()` удваивает лимиты (стр. 35-44).

### 3.9 Отслеживание времени кадра

**Файл:** `crates/horizon-client-render/src/gfx.rs`, поля `Renderer`:
- `last_wait` — время блокировки на GPU/дисплей (стр. 679)
- `last_encode` — CPU время кодирования кадра без ожиданий (стр. 681)
- `drawn_sections`, `section_draws`, `section_draw_calls` (стр. 944-948) — счётчики для измерения

### 3.10 Секционный инстанс-буфер

`instances: wgpu::Buffer` (стр. 827) с данными происхождения секций (origin, fade, visibility), загружается каждый кадр через `uploads.buffer` (стр. 6534-6539). Вместимость растёт степенями двойки (стр. 6493-6501).

---

## 4. Гипотезы (требуют измерений на Adreno 610)

Следующие утверждения **не подтверждены исходниками** и помечены как гипотезы:

1. **Adreno 610 поддерживает `SHADER_EARLY_DEPTH_TEST`** — необходимо для GPU occlusion culling (Vulkan path в `cull.rs`).
2. **Adreno 610 поддерживает `INDIRECT_FIRST_INSTANCE` + `INDIRECT_EXECUTION`** — необходимо для `multi_draw` на Vulkan.
3. **CPU цикл `draw_indexed` по секциям — заметная часть `last_encode`** на Adreno 610 при render distance 8–12.
4. **GPU occlusion culling скроет ≥10–25% вершин секций** на средних дистанциях visibility (аналогично комментарию в `cull.rs` для M4 Pro, но это не доказательство для Adreno).
5. **multi_draw сократит число CPU draw call и вызовов драйвера** на Adreno 610.
6. **OpenGL ES бэкенд на Adreno 610 не поддерживает indirect draws** — поэтому `multi_draw` может быть бесполезен без перехода на Vulkan.
7. **FrameScale (масштабирование кадра) уже снижает нагрузку на заполнение (fillrate)** при render distance выше допустимого; дополнительно снижать разрешение мира не требуется.

---

## 5. Архитектура Voxelix

### 5.1 Принципы

1. **Разделение на независимые модули** — каждый оптимизирует отдельный узкий пункт, можно включать/выключать по отдельности.
2. **Никакого переписывания без измерений** — CPU occlusion не переписывается, GPU occlusion и multi_draw включаются только через feature flag с фолбэком на существующий путь.
3. **Каждая оптимизация отключаема** — через переменную окружения `HORIZON_VOXELIX_<module>` и/или настройку в `horizon-graphics.txt`.
4. **Измеряемость** — каждая оптимизация логирует своё время в `Renderer::last_encode` / `last_wait` и счётчики `section_draw_calls`.

### 5.2 Структура модулей

```
Voxelix (координация, флаги, метрики)
├── V01 MultiDrawIndirect (gfx.rs, cull.rs)          P0
├── V02 GpuOcclusionCulling (gfx.rs, cull.rs)        P0 (гипотеза Adreno)
├── V03 CpuOcclusionGraphTuning (occlusion.rs)       P1
├── V04 MeshUploadBatching (upload.rs, upload_budget.rs) P1
├── V05 ArenaDefragThrottling (arena.rs, gfx.rs)     P1
├── V06 SectionVertexPacking (mesher.rs, gfx.rs)     P2
├── V07 LODTransitionFadeCulling (lod.rs, gfx.rs)    P2
└── V08 AndroidRenderScale (frame_scale.rs)          P2 (уже существует, мониторинг)
```

Координация через структуру `VoxelixConfig` с флагами, читаемыми при инициализации `Renderer` (`gfx.rs`, `Renderer::new`, стр. ~1800+). Каждый модуль инкапсулирован в своём `impl` блоке или отдельном файле в `gfx/`.

### 5.3 Переключатели

| Модуль | Переключатель (env) | Переключатель (config) | Значение по умолчанию |
|---|---|---|---|
| V01 MultiDraw | `HORIZON_VOXELIX_MULTIDRAW=0` | `voxelix.multidraw` | off на Android (как сейчас) |
| V02 GpuOcclusion | `HORIZON_VOXELIX_GPUCULL=0` | `voxelix.gpu_cull` | off на Android (как сейчас) |
| V03 CpuOcclusion | `HORIZON_VOXELIX_CPUCULL=0` | `voxelix.cpu_cull` | on (существующее поведение) |
| V04 UploadBatching | `HORIZON_VOXELIX_UPLOAD=0` | `voxelix.upload` | on |
| V05 ArenaDefrag | `HORIZON_VOXELIX_ARENA=0` | `voxelix.arena` | on |
| V06 VertexPacking | `HORIZON_VOXELIX_VERTEX=0` | `voxelix.vertex` | off (требует изменения формата вершины) |
| V07 LODFadeCull | `HORIZON_VOXELIX_LOD=0` | `voxelix.lod` | off |
| V08 RenderScale | `HORIZON_VOXELIX_SCALE=0` | `voxelix.scale` | on (существующее поведение) |

---

## 6. Модули и оптимизации

### V01 — Multi-Draw Indirect для секций terrain

**Файлы и функции:**
- `crates/horizon-client-render/src/gfx.rs`: `Renderer::new` (стр. ~3597, переменная `multi_draw`), `draw_sections` (стр. 4862-4905), `draw_frame` (стр. 5916+, построение `self.draws` / `self.indirect`)
- `crates/horizon-client-render/src/gfx/cull.rs`: `Cull` структура, взаимодействие с multi-draw (стр. 6556-6568)
- `crates/horizon-client-render/src/gfx/flat.rs`: эмуляция multi-draw (стр. 153, `Flat::wanted`)

**Принцип работы:** Убрать `cfg!(target_os = "android")` из условия `multi_draw`, оставив только проверку фичей устройства (`INDIRECT_FIRST_INSTANCE` + `INDIRECT_EXECUTION`). На Adreno 610 с Vulkan это позволит рисовать группу секций одной командой `multi_draw_indexed_indirect` вместо цикла `draw_indexed` на CPU.

**Ожидаемый эффект (ГИПОТЕЗА):** Снижение CPU времени кодирования кадра (`last_encode`) за счёт меньшего числа вызовов драйвера; уменьшение `section_draw_calls`. Величина неизвестна без замеров на устройстве.

**Риски и совместимость:**
- Если Adreno 610 / драйвер не поддерживает нужные фичи — `multi_draw` остаётся `false`, фолбэк на существующий цикл.
- Возможная регрессия порядка отрисовки непрозрачных секций — group by page сохраняется (`page_runs.rs`).
- Нужен тест на реальном устройстве, прежде чем менять дефолт на `on`.

**Способ измерения:** Сравнить `section_draw_calls` и `last_encode` с `multi_draw = on/off` на одинаковом сниппете мира. Ожидаемое улучшение: меньше вызовов на кадр, стабильнее `last_encode`.

---

### V02 — GPU Occlusion Culling на Adreno 610

**Файлы и функции:**
- `crates/horizon-client-render/src/gfx/cull.rs`: `Cull::supported` (стр. 190-213), `Cull::new` (стр. 215), `Cull::begin` (стр. 362), `Cull::test` / `test_pass` (стр. 499, 534), `Cull::end` / `submitted` (стр. 561, 587)
- `crates/horizon-client-render/src/gfx.rs`: инициализация `self.cull` (стр. 3624-3640), использование в `draw_frame` (стр. 6556-6568)
- `crates/horizon-client-render/src/gfx/cull.wgsl`: шейдер теста боксов секций

**Принцип работы:** Убрать `cfg!(target_os = "android")` и `compatibility()` из фильтров `Cull::supported`, оставить проверку фичей (`SHADER_EARLY_DEPTH_TEST`, `INDIRECT_FIRST_INSTANCE`, `VERTEX_STORAGE`, `FRAGMENT_WRITABLE_STORAGE`). На Vulkan с early depth test — тест боксов секций в фазе 1, фаза 2 рисует только видимые.

**Ожидаемый эффект (ГИПОТЕЗА):** Снижение количества нарисованных вершин секций на 10–30% при достаточном количестве перекрытий (подземелья, густой ландшафт). Снижение GPU времени, но возможный рост CPU из-за readback (результаты читаются с задержкой в несколько кадров).

**Риски и совместимость:**
- Readback результатов задерживает реакцию на 1–2 кадра; при быстром движении камеры возможны артефакты «провала» чанков (старые результаты рисуют скрытую секцию в фазе 1).
- На Metal-подобном бэкенде тест идёт в шейдере (не early), дороже.
- Если `SHADER_EARLY_DEPTH_TEST` не поддерживается Adreno 610 — модуль отключается.
- **Нельзя переписывать существующий CPU occlusion без измерений** — этот модуль дополняет, а не заменяет `SectionOcclusionGraph`.

**Способ измерения:** Сравнить `drawn_sections`, `section_draw_calls`, GPU время (если доступно через `gpu_timer` / `frame_timing`) с `gpu_culling = on/off`. Также отслеживать `Cull::hidden` (доля скрытых индексов).

---

### V03 — Настройка CPU occlusion grafa (существующий)

**Файлы и функции:**
- `crates/horizon-client-render/src/gfx/occlusion.rs`: `SectionOcclusionGraph`, `Graph`, BFS поиск, `REACHED` / `SEES` флаги, `DISTANT_SECTIONS` = 60 блоков (стр. 33)
- `crates/horizon-client-render/src/gfx.rs`: `start_occlusion` (стр. 6360), `self.occlusion` (стр. 888)

**Принцип работы:** Существующий CPU occlusion graph работает вне потока рендеринга. Voxelix не переписывает его, а только добавляет tuning-параметры (радиус поиска, частота полного перестроения, порог `RETEST` в `cull.rs` стр. 43) под Adreno 610.

**Ожидаемый эффект (ГИПОТЕЗА):** Лучшее соотношение между точностью ocultation и CPU затратами на пересчёт графа при быстром перемещении камеры.

**Риски:** Изменение параметров графа может скрыть видимые секции (полное отсутствие отрисовки) — требует визуальной верификации.

**Способ измерения:** Сравнение времени работы `start_occlusion` и доли скрытых секций до/после изменения параметров.

---

### V04 — Пакетная загрузка мешей в GPU-буферы (Upload Batching)

**Файлы и функции:**
- `crates/horizon-client-render/src/gfx/upload.rs`: `Uploads::texture`, `Uploads::buffer`, `StagingBelt` (стр. 1-80+)
- `crates/horizon-client-render/src/gfx/upload_budget.rs`: `UploadBudget::with_frame_state` (стр. 25-58), мобильные лимиты (2–4 MiB, 1–3 мс)
- `crates/horizon-client-render/src/gfx.rs`: `section_uploads` (стр. 818), `section_copies` (стр. 819), `take_section_copies` (стр. 4850)

**Принцип работы:** Существующий `StagingBelt` уже уменьшает число staging-буферов. Voxelix добавляет:
1. Группировку загрузок секций в фрейме по размеру (большие — отдельно, мелкие — пачками).
2. Динамическое ужесточение `UploadBudget` при росте `last_wait` (пропуск кадров загрузки некритичных遠ких секций).
3. Приоритет ближних секций (уже есть в `streaming.rs`, ближайшие zuerst).

**Ожидаемый эффект (ГИПОТЕЗА):** Меньше стॉलлов CPU на ожидании завершения загрузки (`last_wait` снижается), меньше пропусков кадров при быстром полёте.

**Риски:** Задержка появления遠ких чанков на один кадр; при агрессивном бюджете возможны «дыры» в ландшафте.

**Способ измерения:** Сравнить `last_wait`, число `section_uploads` в кадр, визуально проверить отсутствие дыр при движении.

---

### V05 — Дросселирование дефрагментации арены

**Файлы и функции:**
- `crates/horizon-client-render/src/gfx/arena.rs`: `Arena::compact`, `Arena::trim`, `Arena::sparsest`, `Arena::close`
- `crates/horizon-client-render/src/gfx.rs`: `compact_arena` (стр. 4928), бюджет `32 << 20` байт на кадр (стр. 4930)

**Принцип работы:** `compact_arena` перемещает меши с самых разряженных страниц, чтобы вернуть пустые страницы вビデопамять. На Adreno 610 с unified memory fragmentation менее критична, чем на дискретном GPU, но перемещение мешей стоит CPU. Voxelix добавляет условие: дефрагментировать только когда фрагментация превышает порог (например, >25% страниц частично пусты) или когда `last_wait` растёт.

**Ожидаемый эффект (ГИПОТЕЗА):** Снижение CPU нагрузки в стабильных сценах (меньше вызовов `arena.close` / копирования вершин).

**Риски:** При отключенной дефрагментации видео-память может фрагментироваться сильнее, что приведёт к выделению новых страниц и_allocations.

**Способ измерения:** Сравнить `last_encode` и число задействованных страниц арены до/после.

---

### V06 — Упаковка вершин секции (Vertex Packing)

**Файлы и функции:**
- `crates/horizon-client-render/src/mesher.rs`: `TerrainVertex` (стр. 58-70, 24 байта)
- `crates/horizon-client-render/src/gfx.rs`: `TERRAIN_ATTRIBUTES` (стр. 28-29), загрузка вершин в `arena`

**Принцип работы (ГИПОТЕЗА):** `TerrainVertex` 24 байта. Можно рассмотреть более плотный формат (например, упаковка `uv` и `layer` в один `u32`, `light` в `u16` уже есть). Это уменьшит ширину пропуска памяти при загрузке и рендеринге.

**Ожидаемый эффект (ГИПОТЕЗА):** Меньше памяти на буферы секций, выше throughput memory bandwidth на Adreno 610.

**Риски:** Требует изменения формата вершины, шейдеров `terrain.wgsl`, `flat.rs`, `mesher.rs`. Высокий риск регрессии визуального вида. Требует полного регрессионного тестирования. **Откладывается на P2 и только после измерений, подтвердивших, что memory bandwidth — узкое место.**

**Способ измерения:** Сравнить время загрузки мешей и GPU время рендеринга с текущим и упакованным форматом.

---

### V07 — ЛОД-переходы и затухание секций (LOD Fade Culling)

**Файлы и функции:**
- `crates/horizon-client-render/src/gfx/lod.rs`: `lod::Tile`, `lod::instance`, `lod::fade_code`, `lod::HELD`
- `crates/horizon-client-render/src/gfx.rs`: `visible_lods` (стр. 6389-6400), `section_instance` (стр. 309-327), `chunk_visibility` (стр. 330-336)

**Принцип работы (ГИПОТЕЗА):** Секции, затухающие над far terrain (LOD плитки), рисуются даже когда почти невидимы. Voxelix добавляет раннее отсечение секций с `visibility < порог` (например, < 0.05) при отсутствии пересечения с ближней геометрией.

**Ожидаемый эффект (ГИПОТЕЗА):** Меньше рисуемых секций на границе render distance.

**Риски:** Видимые «прыжки» при исчезновении затухающих секций.

**Способ измерения:** Сравнить `drawn_sections` и визуально проверить плавность переходов.

---

### V08 — Android Render Scale (мониторинг существующего)

**Файлы и функции:**
- `crates/horizon-client-render/src/gfx/frame_scale.rs`: `FrameScale::frame`, `FrameScale::present`
- `crates/horizon-client-render/src/gfx.rs`: `frame_scale: Option<frame_scale::FrameScale>` (стр. 894-895), использование в `draw_frame` (стр. 6024-6037, 6095-6098)

**Принцип работы:** Существующий механизм рисует мир в текстуру меньшего размера и масштабирует на экран через post-process копирующий проход. Voxelix не меняет его, а добавляет автоматический подбор коэффициента масштаба на основе `last_wait` / FPS (например, если `last_wait > 16 мс`, снизить масштаб с 1.0 до 0.75).

**Ожидаемый эффект (ГИПОТЕЗА):** Стабильный FPS при динамической нагрузке за счёт снижения fillrate.

**Риски:** Меньшая чёткость изображения при активном масштабировании.

**Способ измерения:** Сравнить FPS и `last_wait` с фиксированным и динамическим масштабом.

---

## 7. Приоритеты

| Приоритет | Модуль | Обоснование |
|---|---|---|
| **P0** | V01 MultiDrawIndirect | Уже частично инфраструктура есть (`multi_draw` поле, `indirect` буфер, `draw_sections` поддерживает оба пути). При подтверждении поддержки фичей Adreno 610 даёт прямой прирост на вызовы драйвера. |
| **P0** | V02 GpuOcclusionCulling | Инфраструктура есть (`cull::Cull`, `cull.wgsl`, readback). Главный вопрос — поддержка Adreno 610. Если фичи есть — большой потенциал. |
| **P1** | V03 CpuOcclusionGraphTuning | Существующий код, только параметры. Низкий риск, но выигрыш ограничен. |
| **P1** | V04 UploadBatching | `upload_budget.rs` уже имеет мобильные лимиты; настройка под Adreno 610 снижает стॉलлы. |
| **P1** | V05 ArenaDefragThrottling | Низкий риск, измеряется через `last_encode`. |
| **P2** | V06 VertexPacking | Высокий риск, требует изменения формата вершины и шейдеров. Только после доказательства, что memory bandwidth — бутылочное горлышко. |
| **P2** | V07 LODFadeCulling | Узкий сценарий (границы render distance), ограниченный выигрыш. |
| **P2** | V08 AndroidRenderScale | Мониторинг существующего;dynamic scale требует обратной связи от игрового цикла (FPS), которой на уровне рендерера нет напрямую. |

---

## 8. Этапный план реализации и тестирования на Adreno 610

### Этап 1 — Измерения и базовый профиль (неделя 1)

Цель: подтвердить или опровергнуть гипотезы о поддерживаемых фичах и узких местах.

1. Запустить Horizon на OPPO CPH2727 с `--resources .local/horizon-client/26.3 --offscreen`.
2. Включить `HORIZON_GPU_TIMER=1` и `frame_timing` (если доступно) для сбора GPU времени.
3. Собрать показатели: `last_encode`, `last_wait`, `section_draw_calls`, `drawn_sections`, `section_uploads`.
4. Проверить `adapter.features()` на наличие `SHADER_EARLY_DEPTH_TEST`, `INDIRECT_FIRST_INSTANCE`, `INDIRECT_EXECUTION`, `VERTEX_STORAGE`, `FRAGMENT_WRITABLE_STORAGE`.
5. Определить бэкенд (Vulkan или OpenGL ES).
6. Зафиксировать базовый FPS и `last_encode` при render distance 8, 12, 16.

**Критерии перехода к этапу 2:** Наличие воспроизводимого профиля; известно, поддерживает ли Adreno 610 indirect draws и early depth test.

### Этап 2 — V01 MultiDrawIndirect (неделя 2)

1. Убрать `cfg!(target_os = "android")` из условия `multi_draw` в `gfx.rs` стр. 3597, оставив только проверку фичей.
2. Добавить env-флаг `HORIZON_VOXELIX_MULTIDRAW` (по умолчанию off на Android до замеров).
3. Сравнить `section_draw_calls` и `last_encode` с флагом on/off.
4. Если прирост подтверждён и нет регрессий — включить по умолчанию на Vulkan+batching.

**Тест:** `python3 tools/lan_e2e.py --check chest` (или эквивалентный рендер-тест), визуальная проверка отсутствия мерцания секций.

### Этап 3 — V02 GpuOcclusionCulling (неделя 3)

1. Убрать `cfg!(target_os = "android")` и `compatibility()` из фильтров `Cull::supported` в `gfx.rs` стр. 3624-3627.
2. Добавить env-флаг `HORIZON_VOXELIX_GPUCULL`.
3. Замерить `Cull::hidden` (доля скрытых индексов), GPU время, `last_encode`.
4. Проверить отсутствие артефактов при быстром повороте камеры (readback задержка).

**Тест:** Сравнение с и без culling в одинаковых сценах (пещера, поверхность,高度различия).

### Этап 4 — V03–V05 (неделя 4–5)

1. V03: Подобрать параметры `occlusion.rs` (`RETEST`, радиус поиска) под мобильный профиль.
2. V04: Настроить `upload_budget.rs` под Adreno 610 (лимиты 2–4 MiB, время 1–3 мс, зависимость от `last_wait`).
3. V05: Добавить порог фрагментации для `compact_arena`.

**Тест:** Сравнение `last_encode`, `last_wait`, число загрузок в кадр.

### Этап 6 — P2 модули только при необходимости (неделя 6+)

V06–V08 реализуются только если этапы 2–5 не дают целевого FPS, и только после подтверждения соответствующего узкого места.

### Каждый релиз — откат

Перед каждым включением по умолчанию проверяется, что все модули отключаются по отдельности через env-флаги, и что при `HORIZON_VOXELIX_ALL=0` поведение идентично базовому Android-билду без Voxelix.

---

## 9. Тестирование и измерение

| Что измерять | Где в коде | Инструмент |
|---|---|---|
| `last_encode` (CPU кодирование кадра) | `gfx.rs` стр. 681, 6130 | `Renderer::last_encode` |
| `last_wait` (GPU/дисплей ожидание) | `gfx.rs` стр. 679, 6129 | `Renderer::last_wait` |
| `section_draw_calls` | `gfx.rs` стр. 948, 6913 | `Renderer::section_draw_calls` |
| `drawn_sections` | `gfx.rs` стр. 944, 6492 | `Renderer::drawn_sections` |
| `Cull::hidden` (доля скрытых) | `cull.rs` стр. 150 | `Cull.hidden` |
| GPU время по этапам | `gfx/timing.rs`, `gfx/frame_timing.rs` | `GpuTimer`, `FrameTiming` |
| FPS | Игровой цикл (`game.rs`) | `horizon-ctl state` / наложение |

Методология: фиксировать мир, позицию камеры, render distance; прогонять N секунд; сравнивать медиану и p95. Не полагаться на одиночные кадры.

---

## 10. Совместимость и откат

- Все модули отключаются по отдельности через `HORIZON_VOXELIX_<module>=0`.
- `HORIZON_VOXELIX_ALL=0` отключает всё Voxelix, поведение идентично базовому Android-билду.
- При неудаче любого модуля на Adreno 610 — откат на существующий путь (per-section `draw_indexed`, CPU occlusion, текущий upload budget).
- Не меняется игровая логика, NBT, протокол, сетевой код.

---

# Первая реализация

**Какую оптимизацию реализовать первой: V01 MultiDrawIndirect.**

**Почему:**
1. Инфраструктура уже существует: поле `multi_draw` в `Renderer`, буфер `indirect`, метод `draw_sections` поддерживает оба пути (цикл `draw_indexed` и `multi_draw_indexed_indirect`). Нужно только убратьartificial `cfg!(target_os = "android")` блокировку и оставить проверку фичей устройства.
2. Это напрямую адресует известное ограничение: на Android каждая секция рисуется отдельным `draw_indexed` даже когда драйвер мог бы принять пакетную команду. На Adreno 610 с Vulkan это потенциально снижает нагрузку на CPU драйвера и `last_encode`.
3. Низкий риск: если фичи не поддерживаются, `multi_draw` остаётся `false`, фолбэк на существующий цикл. Визуальный вид не меняется (порядок отрисовки сохраняется через `page_runs`).
4. Легко измерить: `section_draw_calls` и `last_encode` доступны в `Renderer` и логируются.

**Последующий шаг:** Только после подтверждения, что Adreno 610 действительно поддерживает `INDIRECT_FIRST_INSTANCE` + `INDIRECT_EXECUTION` на Vulkan, и после замеров, показывающих реальное снижение `section_draw_calls` без регрессий, можно приступать к V02 (GPU occlusion).

---

## Приложение: проверка фактов перед созданием документа

| Фактор | Подтверждено источником | Место |
|---|---|---|
| multi_draw выключен на Android | Да | `gfx.rs:3597` |
| GPU occlusion выключен на Android | Да | `cull.rs:191-193`, `gfx.rs:3624-3627` |
| CPU occlusion graph с worker | Да | `occlusion.rs`, `streaming.rs` |
| Android-специфичные модули (frame_scale, opaque_output) | Да | `gfx/frame_scale.rs`, `gfx/opaque_output.rs`, `gfx.rs:213-234` |
| Upload budget с мобильными лимитами | Да | `upload_budget.rs:15-58` |
| `gpu_batching` по умолчанию false на Android | Да | `android_runtime.rs:124,177-179` |
| Adreno 610 поддерживает indirect draws | **Нет (гипотеза)** | Требует `adapter.features()` на устройстве |
| Adreno 610 поддерживает early depth test | **Нет (гипотеза)** | Требует `adapter.features()` на устройстве |
| CPU цикл draw_indexed — узкое место | **Нет (гипотеза)** | Требует замеров `last_encode` |

---

**Документ подготовлен без изменения исходного кода, без сборки и без установки зависимостей. Все утверждения помечены как подтверждённые исходниками или как гипотезы.**
