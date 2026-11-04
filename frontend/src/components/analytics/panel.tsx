import type { ReactNode } from "react";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { planColor, planName } from "./plans";

export interface PanelProps {
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}

export function Panel({
  title,
  description,
  actions,
  children,
  className,
}: PanelProps) {
  return (
    <Card
      className={cn(
        "analytics-panel flex min-w-0 flex-col gap-0 overflow-hidden py-0",
        className,
      )}
    >
      <header className="panel-heading flex flex-wrap items-start justify-between gap-x-4 gap-y-3">
        <div className="min-w-0">
          <h2 className="text-base font-medium leading-snug tracking-[-0.01em]">
            {title}
          </h2>
          {description && (
            <p className="mt-1 text-[13px] leading-5 text-muted-foreground">
              {description}
            </p>
          )}
        </div>
        {actions && (
          <div className="flex max-w-full shrink-0 items-center gap-2">
            {actions}
          </div>
        )}
      </header>
      {children}
    </Card>
  );
}

export function PanelNote({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <p
      className={cn(
        "panel-caption px-4 pb-4 pt-2 sm:px-5",
        className,
      )}
    >
      {children}
    </p>
  );
}

export function EmptyState({
  message = "Для этой выборки пока нет наблюдений",
}: {
  message?: string;
}) {
  return (
    <div className="px-5 py-10 text-center">
      <p className="text-sm font-medium">{message}</p>
      <p className="mt-2 text-[13px] leading-5 text-muted-foreground">
        Выберите другой тариф или расширьте период первой подписки.
      </p>
    </div>
  );
}

export function PlanLabel({ planId }: { planId: string }) {
  return (
    <span className="inline-flex items-center gap-2 whitespace-nowrap">
      <span
        className="size-[7px] shrink-0 rounded-full"
        style={{ backgroundColor: planColor(planId) }}
        aria-hidden="true"
      />
      {planName(planId)}
    </span>
  );
}
