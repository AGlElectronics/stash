type Props = {
  title: string;
  description?: string;
  comingIn?: string;
};

export function PlaceholderPage({ title, description, comingIn }: Props) {
  return (
    <div className="p-6 max-w-5xl">
      <h1 className="text-2xl font-semibold mb-1">{title}</h1>
      {description ? (
        <p className="text-muted-foreground mb-6 text-sm">{description}</p>
      ) : null}
      <div className="rounded-lg border border-dashed border-border bg-card p-8 text-center">
        <p className="text-sm text-muted-foreground">
          {comingIn ? `Lands in ${comingIn}.` : 'Not implemented yet.'}
        </p>
      </div>
    </div>
  );
}
