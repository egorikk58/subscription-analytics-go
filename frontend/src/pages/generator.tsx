import { useEffect, useState, type FormEvent } from "react";
import {
  ArrowUpRight,
  Check,
  Download,
  LoaderCircle,
  Shuffle,
  Sparkles,
} from "lucide-react";
import { Panel } from "@/components/analytics/panel";
import {
  DatasetMetrics,
  DatasetPassport,
  DatasetSamples,
} from "@/components/dataset-samples";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { number } from "@/lib/format";
import { autoUsersCount } from '@/lib/seed';
import type {
  Dashboard,
  SeedConfig,
  DatasetSamples as DatasetSamplesType,
} from "@/types";

export interface GeneratorPageProps {
  report: Dashboard;
  onGenerate: (config: SeedConfig) => Promise<void>;
  isGenerating: boolean;
  isPreview: boolean;
  onExplore: () => void;
  initialConfig?: SeedConfig;
  onConfigChange?: (config: SeedConfig) => void;
  sampleKind?: keyof DatasetSamplesType;
  onSampleKindChange?: (kind: keyof DatasetSamplesType) => void;
}
interface SeedDraft {
  seed: string;
  users: string;
  automatic: boolean;
  months: string;
  date: string;
}
function toDraft(config: SeedConfig): SeedDraft {
  return {
    seed: String(config.seed),
    users: config.users_count == null ? '' : String(config.users_count),
    automatic: config.users_count == null,
    months: String(config.history_months),
    date: config.data_as_of.slice(0, 10),
  };
}
function parseDraft(draft: SeedDraft): SeedConfig | null {
  const seed = Number(draft.seed);
  if (
    draft.seed.trim() === "" ||
    !Number.isInteger(seed) ||
    seed < 0 ||
    seed > 2147483647 ||
    !/^\d{4}-\d{2}-\d{2}$/.test(draft.date) ||
    (!draft.automatic && (!Number.isInteger(Number(draft.users)) || Number(draft.users) < 10000 || Number(draft.users) > 100000))
  )
    return null;
  return {
    seed,
    ...(draft.automatic ? {} : { users_count: Number(draft.users) }),
    history_months: Number(draft.months),
    data_as_of: `${draft.date}T00:00:00Z`,
  };
}

export function GeneratorPage({
  report,
  onGenerate,
  isGenerating,
  isPreview,
  onExplore,
  initialConfig,
  onConfigChange,
  sampleKind,
  onSampleKindChange,
}: GeneratorPageProps) {
  const dataset = report.dataset;
  const [draft, setDraft] = useState<SeedDraft>(() =>
    toDraft(
      initialConfig || {
        seed: dataset.seed ?? 42,
        history_months: dataset.history_months || 24,
        data_as_of: dataset.dataset_as_of || report.data_as_of,
      },
    ),
  );
  const [error, setError] = useState("");
  const [automaticCount, setAutomaticCount] = useState<number>();
  useEffect(() => {
    let current = true;
    const seed = Number(draft.seed);
    if (draft.seed.trim() && Number.isInteger(seed) && seed >= 0 && seed <= 2147483647) {
      void autoUsersCount(seed).then(count => { if (current) setAutomaticCount(count) }).catch(() => { if (current) setAutomaticCount(undefined) });
    }
    return () => { current = false };
  }, [draft.seed]);
  function update(patch: Partial<SeedDraft>) {
    const next = { ...draft, ...patch };
    setDraft(next);
    setError("");
    const config = parseDraft(next);
    if (config) onConfigChange?.(config);
  }
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const config = parseDraft(draft);
    if (!config) {
      setError("Укажите целый seed, размер от 10 000 до 100 000 и дату среза.");
      return;
    }
    setError("");
    onConfigChange?.(config);
    try {
      await onGenerate(config);
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Не удалось сгенерировать набор. Повторите запрос.",
      );
    }
  }
  function randomParameters() {
    const random = crypto.getRandomValues(new Uint32Array(1));
    update({
      seed: String((random[0] ?? 0) % 2147483647),
      automatic: true,
    });
  }
  function downloadPassport() {
    const passport = {
      seed: dataset.seed,
      users_count: dataset.users_count,
      history_months: dataset.history_months,
      data_as_of: dataset.dataset_as_of,
      generator_version: dataset.generator_version,
      fingerprint: dataset.fingerprint,
    };
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(passport, null, 2)], {
        type: "application/json",
      }),
    );
    const link = document.createElement("a");
    link.href = url;
    link.download = `dataset-seed-${dataset.seed}.json`;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  return (
    <div className="space-y-6">
      {isPreview && (
        <div
          className="flex flex-wrap items-center justify-between gap-2 rounded-md bg-primary/5 px-4 py-3 text-[13px]"
          role="status"
        >
          <p className="flex items-center gap-2">
            <Check className="size-4 text-primary" aria-hidden="true" />
            <span>
              Создан seed{" "}
              <strong className="font-medium">{dataset.seed}</strong> ·{" "}
              {number(dataset.users_count)} пользователей
            </span>
          </p>
          <Button variant="ghost" size="sm" onClick={onExplore}>
            Открыть обзор
            <ArrowUpRight className="size-4" />
          </Button>
        </div>
      )}
      <div className="flex flex-wrap items-center justify-between gap-2 text-[13px] text-muted-foreground">

        <Button variant="outline" onClick={downloadPassport}>
          <Download className="size-4" />
          Экспорт параметров
        </Button>
      </div>
      <DatasetMetrics dataset={dataset} />
      <div className="grid items-start gap-6 xl:grid-cols-2">
        <Panel
          title="Новый набор данных"
        >
          <form id="seed-form" onSubmit={submit} className="px-4 pb-5 sm:px-5">
            <fieldset
              disabled={isGenerating}
              className="grid grid-cols-2 gap-x-3 gap-y-5 sm:gap-x-4"
            >
              <div className="col-span-2 space-y-3 rounded-md bg-muted/60 p-3">
                <div className="flex flex-wrap gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={randomParameters}
                  >
                    <Shuffle className="size-4" />
                    Случайные параметры
                  </Button>
                  <Button type="button" variant="outline" size="sm" aria-pressed={draft.automatic} onClick={() => update({ automatic: true })}>По seed</Button>
                  {[25000, 50000, 100000].map((count) => (
                    <Button
                      key={count}
                      type="button"
                      variant="outline"
                      size="sm"
                      aria-pressed={!draft.automatic && Number(draft.users) === count}
                      onClick={() => update({ users: String(count), automatic: false })}
                    >
                      {number(count)} чел.
                    </Button>
                  ))}
                </div>

              </div>
              <div className="min-w-0 space-y-2">
                <Label htmlFor="seed-value" className="text-[13px] font-normal">
                  Seed
                </Label>
                <div className="flex gap-1.5">
                  <Input
                    id="seed-value"
                    name="seed"
                    type="number"
                    className="min-h-11 min-w-0 text-sm sm:min-h-10"
                    min={0}
                    max={2147483647}
                    step={1}
                    value={draft.seed}
                    onChange={(event) => update({ seed: event.target.value })}
                    required
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    className="size-11 shrink-0 sm:size-10"
                    aria-label="Случайный seed"
                    onClick={() =>
                      update({
                        seed: String(
                          (crypto.getRandomValues(new Uint32Array(1))[0] ?? 0) %
                            2147483647,
                        ),
                      })
                    }
                  >
                    <Shuffle className="size-4" />
                  </Button>
                </div>
              </div>
              <div className="min-w-0 space-y-2">
                <div className="flex items-center justify-between gap-2"><Label htmlFor="seed-users" className="text-[13px] font-normal">Пользователей</Label><Button type="button" variant="link" size="sm" className="h-auto p-0 text-xs" onClick={() => update({ automatic: !draft.automatic, users: String(automaticCount ?? dataset.users_count) })}>{draft.automatic ? 'Вручную' : 'По seed'}</Button></div>
                <Input
                  id="seed-users"
                  type="number"
                  min={10000}
                  max={100000}
                  step={1}
                  required={!draft.automatic}
                  disabled={draft.automatic}
                  placeholder="По seed"
                  className="min-h-11 w-full text-sm sm:min-h-10"
                  value={draft.automatic ? automaticCount ?? '' : draft.users}
                  onChange={(event) => update({ users: event.target.value })}
                />
              </div>
              <div className="min-w-0 space-y-2">
                <Label
                  htmlFor="seed-months"
                  className="text-[13px] font-normal"
                >
                  Глубина истории
                </Label>
                <NativeSelect
                  wrapperClassName="w-full"
                  id="seed-months"
                  className="min-h-11 w-full text-sm sm:min-h-10"
                  value={draft.months}
                  onChange={(event) => update({ months: event.target.value })}
                >
                  <NativeSelectOption value="24">24 месяца</NativeSelectOption>
                  <NativeSelectOption value="18">18 месяцев</NativeSelectOption>
                </NativeSelect>
              </div>
              <div className="min-w-0 space-y-2">
                <Label htmlFor="seed-date" className="text-[13px] font-normal">
                  Данные по дату, UTC
                </Label>
                <Input
                  id="seed-date"
                  type="date"
                  className="min-h-11 min-w-0 text-[13px] sm:min-h-10"
                  value={draft.date}
                  onChange={(event) => update({ date: event.target.value })}
                  min="2000-01-01"
                  max={new Date().toISOString().slice(0, 10)}
                  required
                />
              </div>
              <div className="col-span-2">
                <Button
                  id="generate-button"
                  type="submit"
                  className="min-h-11 w-full whitespace-normal sm:min-h-10"
                  disabled={isGenerating}
                >
                  {isGenerating ? (
                    <LoaderCircle
                      className="size-4 animate-spin motion-reduce:animate-none"
                      aria-hidden="true"
                    />
                  ) : (
                    <Sparkles className="size-4" aria-hidden="true" />
                  )}
                  {isGenerating
                    ? "Генерируем записи и считаем метрики…"
                    : "Сгенерировать набор"}
                </Button>
              </div>
            </fieldset>
            {error && (
              <p
                className="mt-4 rounded-md bg-destructive/5 p-3 text-[13px] text-destructive"
                role="alert"
              >
                {error}
              </p>
            )}

          </form>
        </Panel>
        <DatasetPassport dataset={dataset} isPreview={isPreview} />
      </div>
      <DatasetSamples
        dataset={dataset}
        sampleKind={sampleKind}
        onSampleKindChange={onSampleKindChange}
      />
    </div>
  );
}
