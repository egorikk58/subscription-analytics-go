import { useState } from "react";
import { TelegramLookup } from '@/components/telegram-lookup';
import { ArrowDownLeft, ArrowLeft, ArrowRight, Search } from "lucide-react";
import { Panel } from "@/components/analytics/panel";
import { PlanName } from "@/components/dataset-samples";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { date, money, number } from "@/lib/format";
import type { Dashboard, UserRow, UserSort } from "@/types";

export interface UsersPageProps {
  report: Dashboard;
  isFetching?: boolean;
  search: string;
  sort: UserSort;
  onSearchChange: (value: string) => void;
  onSortChange: (value: UserSort) => void;
  onPageChange: (page: number) => void;
  selectedIds: Set<string>;
  onSelectionChange: (ids: Set<string>) => void;
}

export function UsersPage({
  report,
  isFetching = false,
  search,
  sort,
  onSearchChange,
  onSortChange,
  onPageChange,
  selectedIds,
  onSelectionChange,
}: UsersPageProps) {
  const [detail, setDetail] = useState<UserRow | null>(null);
  const data = report.users;
  const checkedCount = data.items.filter((user) =>
    selectedIds.has(user.id),
  ).length;
  const allChecked =
    data.items.length > 0 && checkedCount === data.items.length;
  const changeUser = (id: string, checked: boolean) => {
    const next = new Set(selectedIds);
    if (checked) next.add(id);
    else next.delete(id);
    onSelectionChange(next);
  };
  const changePage = (checked: boolean) => {
    const next = new Set(selectedIds);
    for (const user of data.items) {
      if (checked) next.add(user.id);
      else next.delete(user.id);
    }
    onSelectionChange(next);
  };
  const pageCheckbox = (
    <Checkbox
      id="select-page"
      aria-label="Выбрать страницу"
      checked={allChecked || (checkedCount > 0 ? "indeterminate" : false)}
      onCheckedChange={(value) => changePage(value === true)}
      disabled={isFetching || !data.items.length}
    />
  );
  const empty = (
    <div className="px-5 py-12 text-center">
      <p className="font-medium">Ничего не найдено</p>

    </div>
  );
  return (
    <>
      {report.mode === 'live' && <div className="mb-6"><TelegramLookup initialID={String(data.items[0]?.telegram_id ?? '')}/></div>}
      <Panel
        title="Пользователи выборки"
        actions={
          <Badge variant="secondary" className="rounded-sm font-normal">
            {report.mode === "preview" ? "Предпросмотр" : "Снимок данных"}
          </Badge>
        }
      >
        <div className="flex flex-col gap-3 px-4 pb-4 sm:flex-row sm:justify-between sm:px-5">
          <label className="relative block min-w-0 sm:w-80">
            <Search
              className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
              aria-hidden="true"
            />
            <Input
              id="user-search"
              className="min-h-11 pl-9 text-sm sm:min-h-9"
              type="search"
              placeholder="Поиск по ID или Telegram ID"
              aria-label="Поиск пользователей"
              value={search}
              onChange={(event) => onSearchChange(event.target.value)}
              maxLength={100}
            />
          </label>
          <NativeSelect
            wrapperClassName="w-full sm:w-auto"
            id="user-sort"
            className="min-h-11 w-full text-sm sm:min-h-9 sm:w-60"
            aria-label="Сортировка"
            value={sort}
            onChange={(event) => {
              const value = event.target.value;
              if (
                value === "ltv_desc" ||
                value === "ltv_asc" ||
                value === "joined_desc"
              )
                onSortChange(value);
            }}
          >
            <NativeSelectOption value="ltv_desc">
              LTV: по убыванию
            </NativeSelectOption>
            <NativeSelectOption value="ltv_asc">
              LTV: по возрастанию
            </NativeSelectOption>
            <NativeSelectOption value="joined_desc">
              Сначала новые подписки
            </NativeSelectOption>
          </NativeSelect>
        </div>
        <div className="hidden md:block">
          <Table className="users-table">
            <TableHeader>
              <TableRow>
                <TableHead className="w-12 pl-5">{pageCheckbox}</TableHead>
                <TableHead className="font-normal">Пользователь</TableHead>
                <TableHead className="font-normal">Первая подписка</TableHead>
                <TableHead className="font-normal">Первый тариф</TableHead>
                <TableHead className="text-right font-normal">
                  Пожизненный LTV
                </TableHead>
                <TableHead className="w-12">
                  <span className="sr-only">Подробнее</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.items.map((user) => (
                <TableRow
                  key={user.id}
                  data-state={selectedIds.has(user.id) ? "selected" : undefined}
                >
                  <TableCell className="pl-5">
                    <Checkbox
                      aria-label={`Выбрать ${user.id}`}
                      data-user-check={user.id}
                      checked={selectedIds.has(user.id)}
                      disabled={isFetching}
                      onCheckedChange={(value) =>
                        changeUser(user.id, value === true)
                      }
                    />
                  </TableCell>
                  <TableCell className="py-3">
                    <span className="font-mono text-xs" title={user.id}>
                      {user.id.slice(0, 8)}…{user.id.slice(-4)}
                    </span>
                    <p className="mt-1 text-xs text-muted-foreground">
                      Telegram {user.telegram_id}
                    </p>
                  </TableCell>
                  <TableCell className="text-[13px]">
                    {date(user.first_subscription_at)}
                  </TableCell>
                  <TableCell className="text-[13px]">
                    <PlanName planId={user.first_plan_id} />
                  </TableCell>
                  <TableCell className="text-right text-[13px] font-medium tabular-nums">
                    {money(user.ltv_minor, 2)}
                  </TableCell>
                  <TableCell className="pr-5">
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={`Подробности пользователя ${user.id}`}
                      onClick={() => setDetail(user)}
                    >
                      <ArrowDownLeft className="size-4 rotate-180" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {!data.items.length && empty}
        </div>
        <div className="md:hidden">
          <label className="flex min-h-12 items-center gap-3 border-y border-border px-4 text-[13px]">
            <Checkbox
              aria-label="Выбрать страницу"
              checked={
                allChecked || (checkedCount > 0 ? "indeterminate" : false)
              }
              onCheckedChange={(value) => changePage(value === true)}
              disabled={isFetching || !data.items.length}
            />
            Выбрать страницу
          </label>
          {data.items.map((user) => (
            <div
              key={user.id}
              className="grid grid-cols-[24px_minmax(0,1fr)_auto] items-center gap-3 border-b border-border px-4 py-3"
            >
              <label className="flex min-h-11 items-center">
                <Checkbox
                  aria-label={`Выбрать ${user.id}`}
                  checked={selectedIds.has(user.id)}
                      disabled={isFetching}
                  onCheckedChange={(value) =>
                    changeUser(user.id, value === true)
                  }
                />
              </label>
              <Button
                variant="ghost"
                className="h-auto min-w-0 flex-col items-start gap-1 px-0 py-1 text-left hover:bg-transparent"
                onClick={() => setDetail(user)}
                aria-label={`Подробности пользователя ${user.id}`}
              >
                <span className="font-mono text-xs">
                  {user.id.slice(0, 8)}…{user.id.slice(-4)}
                </span>
                <span className="text-xs font-normal text-muted-foreground">
                  Telegram {user.telegram_id}
                </span>
                <span className="text-xs font-normal text-muted-foreground">
                  {date(user.first_subscription_at)}
                </span>
              </Button>
              <div className="min-w-0 text-right">
                <p className="whitespace-nowrap text-[13px] font-medium tabular-nums">
                  {money(user.ltv_minor, 2)}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  <PlanName planId={user.first_plan_id} />
                </p>
              </div>
            </div>
          ))}
          {!data.items.length && empty}
        </div>
        <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border px-4 py-3 text-[13px] text-muted-foreground sm:px-5">
          <span>
            {number(data.total)} записей
            {selectedIds.size > 0 && (
              <span id="selected-count">
                {" "}
                · выбрано {number(selectedIds.size)}
              </span>
            )}
          </span>
          <div className="flex items-center gap-3">
            <Button
              variant="outline"
              size="icon"
              aria-label="Предыдущая страница"
              data-page={data.page - 1}
              disabled={isFetching || data.page <= 1}
              onClick={() => onPageChange(data.page - 1)}
            >
              <ArrowLeft className="size-4" />
            </Button>
            <span className="min-w-12 text-center tabular-nums">
              {data.page} / {Math.max(data.pages, 1)}
            </span>
            <Button
              variant="outline"
              size="icon"
              aria-label="Следующая страница"
              data-page={data.page + 1}
              disabled={isFetching || data.page >= data.pages}
              onClick={() => onPageChange(data.page + 1)}
            >
              <ArrowRight className="size-4" />
            </Button>
          </div>
        </div>
      </Panel>
      <Dialog
        open={detail !== null}
        onOpenChange={(open) => {
          if (!open) setDetail(null);
        }}
      >
        <DialogContent className="max-h-[85dvh] overflow-auto">
          <DialogHeader>
            <DialogTitle>Пользователь</DialogTitle>
            <DialogDescription className="break-all font-mono text-xs">
              {detail?.id}
            </DialogDescription>
          </DialogHeader>
          {detail && (
            <dl className="grid gap-4 text-sm">
              {[
                ["Telegram ID", String(detail.telegram_id)],
                ["Первая подписка", date(detail.first_subscription_at)],
                [
                  "Первый тариф",
                  <PlanName key="plan" planId={detail.first_plan_id} />,
                ],
                ["Пожизненный LTV", money(detail.ltv_minor, 2)],
                ["UTM", detail.utm_link || "—"],
              ].map(([label, value]) => (
                <div
                  key={String(label)}
                  className="grid grid-cols-[135px_1fr] gap-3"
                >
                  <dt className="text-muted-foreground">{label}</dt>
                  <dd className="break-all">{value}</dd>
                </div>
              ))}
            </dl>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
