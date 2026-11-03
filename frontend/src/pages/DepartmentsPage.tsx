import { useQuery } from '@tanstack/react-query';
import { createDepartment, deleteDepartment, listDepartments } from '@/api/departments';
import { SimpleResourcePage } from '@/components/SimpleListPage';

export function DepartmentsPage() {
  const q = useQuery({ queryKey: ['departments'], queryFn: listDepartments });

  return (
    <SimpleResourcePage
      title="Departments"
      description="Owners and recipients of inventory."
      resourceLabel="department"
      queryKey={['departments']}
      query={q.refetch}
      rows={q.data?.items}
      isLoading={q.isLoading}
      isError={q.isError}
      onCreate={(body) => createDepartment(body)}
      onDelete={(id) => deleteDepartment(id)}
    />
  );
}
