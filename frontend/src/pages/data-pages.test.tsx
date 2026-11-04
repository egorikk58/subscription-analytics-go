import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { webcrypto } from 'node:crypto';
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DatasetSamples } from "@/components/dataset-samples";
import { GeneratorPage, type GeneratorPageProps } from "@/pages/generator";
import { UsersPage } from "@/pages/users";
import type { Dashboard, SeedConfig, UserSort } from "@/types";

beforeEach(() => { vi.stubGlobal('crypto', webcrypto) });
afterEach(() => { cleanup(); vi.unstubAllGlobals() });

const config: SeedConfig = {
  seed: 42,
  users_count: 10000,
  history_months: 24,
  data_as_of: "2026-11-01T00:00:00Z",
};
const firstId = "11111111-1111-4111-8111-111111111111";
const secondId = "22222222-2222-4222-8222-222222222222";
const paymentId = "33333333-3333-4333-8333-333333333333";
const refundId = "44444444-4444-4444-8444-444444444444";
const report: Dashboard = {
  data_as_of: "2026-11-02T00:00:00Z",
  snapshot_id: 1,
  mode: "live",
  selection: "first_plan",
  summary: {
    currency: "RUB",
    users_count: 2,
    total_revenue_minor: 99000,
    mean_ltv_minor: "49500.00",
    median_ltv_minor: "49500.00",
  },
  plans: [],
  timeline: [],
  retention: [],
  cohorts: [],
  distribution: [],
  users: {
    page: 1,
    page_size: 20,
    pages: 1,
    total: 2,
    items: [
      {
        id: firstId,
        telegram_id: 7400000001,
        first_subscription_at: "2025-01-31T00:00:00Z",
        first_plan_id: "monthly",
        ltv_minor: 99000,
        utm_link: "telegram",
      },
      {
        id: secondId,
        telegram_id: 7400000002,
        first_subscription_at: null,
        first_plan_id: "",
        ltv_minor: 0,
        utm_link: "",
      },
    ],
  },
  dataset: {
    seed: 42,
    generator_version: "v1",
    history_months: 24,
    dataset_as_of: "2026-11-01T00:00:00Z",
    users_count: 10000,
    subscriptions_count: 9500,
    payments_count: 15000,
    succeeded_count: 14000,
    failed_count: 800,
    refunded_count: 200,
    switches_count: 50,
    unpaid_users: 500,
    active_periods: 300,
    fingerprint: "abcdef0123456789",
    samples: {
      users: [
        {
          id: firstId,
          telegram_id: 7400000001,
          created_at: "2024-11-01T00:00:00Z",
          updated_at: "2024-11-01T00:00:00Z",
          utm_link: "telegram",
        },
      ],
      subscriptions: [
        {
          id: "subscription-id",
          user_id: firstId,
          plan_id: "monthly",
          status: "active",
          started_at: "2025-01-31T00:00:00Z",
          current_period_end: "2025-02-28T00:00:00Z",
          canceled_at: null,
          updated_at: "2025-01-31T00:00:00Z",
        },
      ],
      payments: [
        {
          id: paymentId,
          user_id: firstId,
          subscription_id: "subscription-id",
          amount: 99000,
          currency: "RUB",
          status: "succeeded",
          period_start: "2025-01-31T00:00:00Z",
          period_end: "2025-02-28T00:00:00Z",
          created_at: "2025-01-31T00:00:00Z",
          updated_at: "2025-01-31T00:00:00Z",
        },
        {
          id: refundId,
          user_id: firstId,
          subscription_id: "subscription-id",
          amount: 99000,
          currency: "RUB",
          status: "refunded",
          period_start: "2025-01-31T00:00:00Z",
          period_end: "2025-02-28T00:00:00Z",
          created_at: "2025-02-01T00:00:00Z",
          updated_at: "2025-02-01T00:00:00Z",
          refund_of_payment_id: paymentId,
        },
      ],
    },
  },
};

function generatorProps(
  overrides: Partial<GeneratorPageProps> = {},
): GeneratorPageProps {
  return {
    report,
    onGenerate: vi.fn().mockResolvedValue(undefined),
    isGenerating: false,
    isPreview: false,
    onExplore: vi.fn(),
    ...overrides,
  };
}

describe("GeneratorPage", () => {
  it("keeps the draft through a viewport resize and submits the chosen UTC parameters", async () => {
    const user = userEvent.setup(),
      onGenerate = vi.fn().mockResolvedValue(undefined),
      onConfigChange = vi.fn();
    render(
      <GeneratorPage {...generatorProps({ onGenerate, onConfigChange })} />,
    );
    const seed = screen.getByLabelText("Seed", { selector: "input" });
    await user.clear(seed);
    await user.type(seed, "731");
    await user.click(screen.getByRole('button', {name:'Вручную', exact:true}));
    await user.clear(screen.getByLabelText("Пользователей"));
    await user.type(screen.getByLabelText("Пользователей"), "15000");
    fireEvent.resize(window);
    expect(seed).toHaveValue(731);
    expect(screen.getByLabelText("Пользователей")).toHaveValue(15000);
    await user.click(
      screen.getByRole("button", { name: "Сгенерировать набор" }),
    );
    const expected = { ...config, seed: 731, users_count: 15000 };
    expect(onConfigChange).toHaveBeenLastCalledWith(expected);
    expect(onGenerate).toHaveBeenCalledExactlyOnceWith(expected);
  });
  it("restores a valid parent-owned draft after remounting", async () => {
    const user = userEvent.setup();
    let draft: SeedConfig = config;
    const props = generatorProps({
      initialConfig: draft,
      onConfigChange: (next) => {
        draft = next;
      },
    });
    const view = render(<GeneratorPage {...props} />);
    await user.clear(screen.getByLabelText("Seed", { selector: "input" }));
    await user.type(
      screen.getByLabelText("Seed", { selector: "input" }),
      "732",
    );
    await user.clear(screen.getByLabelText("Пользователей"));
    await user.type(screen.getByLabelText("Пользователей"), "20000");
    view.unmount();
    render(<GeneratorPage {...generatorProps({ initialConfig: draft })} />);
    expect(screen.getByLabelText("Seed", { selector: "input" })).toHaveValue(
      732,
    );
    expect(screen.getByLabelText("Пользователей")).toHaveValue(20000);
  });
  it("disables parameter edits while generation is pending", () => {
    render(<GeneratorPage {...generatorProps({ isGenerating: true })} />);
    expect(screen.getByLabelText("Seed", { selector: "input" })).toBeDisabled();
    expect(screen.getByLabelText("Пользователей")).toBeDisabled();
    expect(
      screen.getByRole("button", {
        name: "Генерируем записи и считаем метрики…",
      }),
    ).toBeDisabled();
  });
});

describe("Larger reproducible datasets", () => {
  it("fills random parameters inside limits and only generates on submit", async () => {
    const user = userEvent.setup();
    const onGenerate = vi.fn().mockResolvedValue(undefined);
    render(<GeneratorPage {...generatorProps({ onGenerate })} />);
    await user.click(
      screen.getByRole("button", { name: "Случайные параметры" }),
    );
    const input = screen.getByLabelText("Пользователей") as HTMLInputElement;
    await waitFor(() => expect(Number(input.value)).toBeGreaterThanOrEqual(25000));
    const count = Number(input.value);
    expect(count).toBeGreaterThanOrEqual(25000);
    expect(count).toBeLessThanOrEqual(100000);
    expect(onGenerate).not.toHaveBeenCalled();
    await user.click(
      screen.getByRole("button", { name: "Сгенерировать набор", exact: true }),
    );
    expect(input).toBeDisabled();
    const submitted = onGenerate.mock.calls[0]?.[0] as SeedConfig;
    expect(submitted.users_count).toBeUndefined();
    expect(submitted.seed).toBe(Number(screen.getByLabelText('Seed', {selector:'input'}).getAttribute('value')));
  });
});

describe("UsersPage", () => {
  it("reports controlled search, sorting, selection and opens accessible details", async () => {
    const user = userEvent.setup(),
      onSearch = vi.fn(),
      onSort = vi.fn(),
      onSelect = vi.fn();
    function Harness() {
      const [search, setSearch] = useState("");
      const [sort, setSort] = useState<UserSort>("ltv_desc");
      const [selected, setSelected] = useState(new Set<string>());
      return (
        <UsersPage
          report={report}
          search={search}
          sort={sort}
          onSearchChange={(value) => {
            setSearch(value);
            onSearch(value);
          }}
          onSortChange={(value) => {
            setSort(value);
            onSort(value);
          }}
          onPageChange={vi.fn()}
          selectedIds={selected}
          onSelectionChange={(value) => {
            setSelected(value);
            onSelect(value);
          }}
        />
      );
    }
    const { container } = render(<Harness />);
    await user.type(screen.getByLabelText("Поиск пользователей"), "7400");
    expect(onSearch).toHaveBeenLastCalledWith("7400");
    expect(screen.getByLabelText("Поиск пользователей")).toHaveValue("7400");
    await user.selectOptions(screen.getByLabelText("Сортировка"), "ltv_asc");
    expect(onSort).toHaveBeenLastCalledWith("ltv_asc");
    const table = container.querySelector(".users-table");
    expect(table).not.toBeNull();
    const tableView = within(table as HTMLElement);
    await user.click(
      tableView.getByRole("checkbox", { name: `Выбрать ${firstId}` }),
    );
    expect(onSelect).toHaveBeenLastCalledWith(new Set([firstId]));
    await user.click(
      tableView.getByRole("checkbox", { name: "Выбрать страницу" }),
    );
    expect(onSelect).toHaveBeenLastCalledWith(new Set([firstId, secondId]));
    await user.click(
      tableView.getByRole("button", {
        name: `Подробности пользователя ${firstId}`,
      }),
    );
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText(firstId)).toBeInTheDocument();
    expect(within(dialog).getByText("7400000001")).toBeInTheDocument();
    expect(within(dialog).getByText("990,00 ₽")).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});

it("shows the selected refund with its original payment reference in JSON", async () => {
  const user = userEvent.setup(),
    { container } = render(<DatasetSamples dataset={report.dataset} />);
  await user.click(
    screen.getByRole("button", { name: `Показать JSON ${refundId}` }),
  );
  const json = container.querySelector("pre");
  expect(json?.textContent).toContain('"status": "refunded"');
  expect(json?.textContent).toContain(`"refund_of_payment_id": "${paymentId}"`);
  await user.click(screen.getByRole("tab", { name: "Пользователи" }));
  expect(screen.getByRole("tab", { name: "Пользователи" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  expect(container.querySelector("pre")?.textContent).toContain(
    '"telegram_id": 7400000001',
  );
});
