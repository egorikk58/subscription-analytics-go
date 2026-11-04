import { Panel } from '@/components/analytics/panel'

const tools = [
  {title:'Swagger / OpenAPI',href:'/docs/'},
  {title:'pgAdmin',href:'/db/'},
  {title:'Grafana',href:'/grafana/d/subscope-overview'},
  {title:'Prometheus',href:'/prometheus/'},
]
export function OperationsLinks(){
 return <Panel title="Инструменты"><div className="grid gap-3 px-4 pb-5 sm:grid-cols-2 sm:px-5">{tools.map(tool=><a key={tool.href} href={tool.href} target="_blank" rel="noopener noreferrer" className="rounded-md border border-border p-4 hover:bg-muted/60 focus-visible:outline-2 focus-visible:outline-ring"><span className="text-sm font-medium">{tool.title} ↗</span></a>)}</div></Panel>
}
