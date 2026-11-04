import { Panel } from '@/components/analytics/panel'

const tools = [
  {title:'Swagger / OpenAPI',href:'/docs/',text:'Контракт API, модели, примеры и выполнение запросов.'},
  {title:'pgAdmin',href:'/db/',text:'PostgreSQL: схемы, таблицы и Query Tool. Подключение demo_viewer уже настроено.'},
  {title:'Grafana',href:'/grafana/d/subscope-overview',text:'Обзор сервиса, HTTP/Go, синхронизация/кеш и PostgreSQL/Redis.'},
  {title:'Prometheus',href:'/prometheus/',text:'Запросы PromQL, цели сбора и состояния alert rules.'},
]
export function OperationsLinks(){
 return <Panel title="Служебные инструменты" description="Swagger доступен в базовом стеке. Панели БД и мониторинга — с профилем observability и учётной записью оператора."><div className="grid gap-3 px-4 pb-5 sm:grid-cols-2 sm:px-5">{tools.map(tool=><a key={tool.href} href={tool.href} target="_blank" rel="noopener noreferrer" className="rounded-md border border-border p-4 hover:bg-muted/60 focus-visible:outline-2 focus-visible:outline-ring"><span className="text-sm font-medium">{tool.title} ↗</span><p className="mt-2 text-[13px] leading-relaxed text-muted-foreground">{tool.text}</p></a>)}</div></Panel>
}
