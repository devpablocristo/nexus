type Props = {
  error: Error | null;
  onRetry?: () => void;
};

export function QueryError({ error, onRetry }: Props) {
  if (!error) return null;
  return (
    <div className="error-banner">
      <strong>Failed to load data</strong>
      <p>{error.message}</p>
      {onRetry && <button onClick={onRetry}>Retry</button>}
    </div>
  );
}
