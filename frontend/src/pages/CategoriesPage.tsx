import { useQuery } from '@tanstack/react-query';
import { createCategory, deleteCategory, listCategories } from '@/api/categories';
import { SimpleResourcePage } from '@/components/SimpleListPage';

export function CategoriesPage() {
  const q = useQuery({ queryKey: ['categories'], queryFn: listCategories });

  return (
    <SimpleResourcePage
      title="Categories"
      description="Group items by what they are."
      resourceLabel="category"
      queryKey={['categories']}
      query={q.refetch}
      rows={q.data?.items}
      isLoading={q.isLoading}
      isError={q.isError}
      onCreate={(body) => createCategory(body)}
      onDelete={(id) => deleteCategory(id)}
    />
  );
}
