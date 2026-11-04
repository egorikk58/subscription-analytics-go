// Source: official shadcn/ui new-york-v4 registry, added with shadcn@4.23.0.
// Project adaptation: local cn utility and flat surfaces.
import { cn } from "@/lib/utils"

function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      className={cn("animate-pulse rounded-md bg-accent", className)}
      {...props}
    />
  )
}

export { Skeleton }
