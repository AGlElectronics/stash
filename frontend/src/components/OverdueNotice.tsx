import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { BellRing } from 'lucide-react';
import { getNotifications } from '@/api/notifications';

/** In-app reminder: how many sign-outs are past their expected return. */
export function OverdueNotice() {
  const q = useQuery({
    queryKey: ['notifications'],
    queryFn: getNotifications,
    refetchInterval: 60_000,
    retry: false,
  });
  const overdue = q.data?.overdue_signouts ?? [];
  if (overdue.length === 0) return null;
  const first = overdue[0];
  return (
    <Link
      to={overdue.length === 1 ? '/signouts/$id' : '/dashboard'}
      params={overdue.length === 1 ? { id: first.id } : undefined}
      className="w-full flex items-center gap-2 text-left text-sm px-3 py-2 rounded-pill bg-warning/15 text-warning hover:bg-warning/25 transition-colors"
      title={overdue.map((o) => `${o.signout_number} → ${o.to_department_name}`).join('\n')}
    >
      <BellRing className="w-4 h-4 shrink-0" />
      {overdue.length === 1
        ? `${first.signout_number} is overdue`
        : `${overdue.length} sign-outs overdue`}
    </Link>
  );
}
