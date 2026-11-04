import { planColors, planNames } from "@/lib/format";

export function planName(planId: string): string {
  return (
    planNames[planId as keyof typeof planNames] || planId || "Без подписки"
  );
}

export function planColor(planId: string): string {
  return planColors[planId as keyof typeof planColors] || "#788184";
}
