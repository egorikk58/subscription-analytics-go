// @vitest-environment jsdom
import "@testing-library/jest-dom/vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { TooltipProvider } from "@/components/ui/tooltip";
import type { CohortRow, PeriodRow, TimelineRow } from "@/types";
import { CohortHeatmap } from "./heatmap";
import { RetentionChart, TimelineChart } from "./charts";

beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue(
    new DOMRect(0, 0, 640, 210),
  );
  vi.stubGlobal(
    "ResizeObserver",
    class {
      callback: ResizeObserverCallback;
      constructor(callback: ResizeObserverCallback) {
        this.callback = callback;
      }
      observe(target: Element) {
        this.callback(
          [
            {
              target,
              contentRect: new DOMRect(0, 0, 640, 210),
            } as ResizeObserverEntry,
          ],
          this as unknown as ResizeObserver,
        );
      }
      unobserve() {}
      disconnect() {}
    },
  );
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function fixturePeriods(): PeriodRow[] {
  return [
    {
      period: 1,
      eligible: 10,
      renewed: 5,
      cumulative_eligible: 40,
      rate: 0.5,
      cumulative_rate: 0.125,
    },
    {
      period: 2,
      eligible: 5,
      renewed: 1,
      cumulative_eligible: 40,
      rate: 0.2,
      cumulative_rate: 0.025,
    },
    ...Array.from({ length: 10 }, (_, index) => ({
      period: index + 3,
      eligible: 0,
      renewed: 0,
      cumulative_eligible: 0,
      rate: null,
      cumulative_rate: null,
    })),
  ];
}

const cohorts: CohortRow[] = [
  { month: "2025-01", users_count: 40, periods: fixturePeriods() },
  { month: "2025-02", users_count: 40, periods: fixturePeriods() },
];

describe("cohort denominator and keyboard evidence", () => {
  it("distinguishes conditional and mature-original denominators in accessible evidence", () => {
    const onMonthClick = vi.fn();
    const { rerender } = render(
      <TooltipProvider>
        <CohortHeatmap
          rows={cohorts}
          metric="rate"
          planId="monthly"
          onMonthClick={onMonthClick}
        />
      </TooltipProvider>,
    );
    expect(
      screen.getByRole("button", {
        name: /янв.*продление 1.*50\s*%.*Дошли и завершили: 10/,
      }),
    ).toBeInTheDocument();
    rerender(
      <TooltipProvider>
        <CohortHeatmap
          rows={cohorts}
          metric="cumulative_rate"
          planId="monthly"
          onMonthClick={onMonthClick}
        />
      </TooltipProvider>,
    );
    expect(
      screen.getByRole("button", {
        name: /янв.*продление 1.*12,5\s*%.*Зрелая исходная когорта: 40/,
      }),
    ).toBeInTheDocument();
  });

  it("keeps a single matrix tab stop across horizontal and vertical navigation and opens the selected month", async () => {
    const user = userEvent.setup();
    const onMonthClick = vi.fn();
    render(
      <TooltipProvider>
        <CohortHeatmap
          rows={cohorts}
          metric="rate"
          planId="monthly"
          onMonthClick={onMonthClick}
        />
      </TooltipProvider>,
    );
    const first = screen.getByRole("button", { name: /янв.*продление 1\./ });
    act(() => first.focus());
    await user.keyboard("{ArrowRight}{ArrowDown}");
    const target = screen.getByRole("button", { name: /февр.*продление 2\./ });
    expect(target).toHaveFocus();
    expect(document.querySelectorAll('.heat-cell[tabindex="0"]')).toHaveLength(
      1,
    );
    await user.keyboard("{Enter}");
    expect(onMonthClick).toHaveBeenCalledWith("2025-02");
  });

  it("retains zero percentages and explains missing observations separately", () => {
    const row: CohortRow = {
      month: "2025-03",
      users_count: 1,
      periods: [
        {
          period: 1,
          eligible: 1,
          renewed: 0,
          cumulative_eligible: 1,
          rate: 0,
          cumulative_rate: 0,
        },
        {
          period: 2,
          eligible: 0,
          renewed: 0,
          cumulative_eligible: 0,
          rate: null,
          cumulative_rate: null,
        },
      ],
    };
    render(
      <TooltipProvider>
        <CohortHeatmap
          rows={[row]}
          metric="rate"
          planId="monthly"
          onMonthClick={vi.fn()}
        />
      </TooltipProvider>,
    );
    expect(
      screen.getByRole("button", { name: /продление 1\./ }),
    ).toHaveTextContent("0%");
    expect(
      screen.getByRole("button", { name: /продление 2\./ }),
    ).toHaveTextContent("—");
  });
});

describe("timeline drilldown", () => {
  it('places quarterly renewals at calendar months 3 and 6 and reads their actual denominators', async () => {
    const user = userEvent.setup();
    render(<RetentionChart calendarMonths reports={[{plan_id:'quarterly', cohort_size:40, periods:fixturePeriods()}]} metric="cumulative_rate" onMetricChange={vi.fn()} maxPeriods={2}/>);
    const graph = screen.getByTestId('retention-chart');
    act(() => graph.focus());
    expect(screen.getByTestId('retention-selection')).toHaveTextContent(/3 мес.*Квартальный.*12,5\s*%.*40/);
    await user.keyboard('{ArrowRight}');
    expect(screen.getByTestId('retention-selection')).toHaveTextContent(/6 мес.*Квартальный.*2,5\s*%.*40/);
  });
  it("opens the keyboard-selected month while reading exact monetary values", async () => {
    const user = userEvent.setup();
    const rows: TimelineRow[] = [
      {
        month: "2025-01",
        currency: "RUB",
        users_count: 4,
        total_revenue_minor: 100000,
        mean_ltv_minor: "25000.00",
        median_ltv_minor: "25000.00",
      },
      {
        month: "2025-02",
        currency: "RUB",
        users_count: 2,
        total_revenue_minor: 60000,
        mean_ltv_minor: "30000.00",
        median_ltv_minor: "30000.00",
      },
    ];
    const onMonthClick = vi.fn();
    render(
      <TimelineChart
        rows={rows}
        metric="total_revenue_minor"
        onMetricChange={vi.fn()}
        onMonthClick={onMonthClick}
      />,
    );
    const graph = screen.getByTestId("timeline-chart");
    act(() => graph.focus());
    await user.keyboard("{ArrowRight}");
    expect(screen.getByTestId("timeline-selection")).toHaveTextContent(
      /февр.*600\s*₽.*2 пользователей/,
    );
    await user.keyboard("{Enter}");
    expect(onMonthClick).toHaveBeenCalledWith("2025-02");
  });
});
