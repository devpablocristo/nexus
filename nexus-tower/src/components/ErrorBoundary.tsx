import { Component, type ReactNode } from 'react';

type Props = { children: ReactNode };
type State = { error: Error | null; errorId: string };

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null, errorId: '' };

  static getDerivedStateFromError(error: Error): State {
    return { error, errorId: buildErrorID() };
  }

  render() {
    if (this.state.error) {
      const reportURL = `mailto:support@nexus.io?subject=${encodeURIComponent(`Tower issue ${this.state.errorId}`)}&body=${encodeURIComponent(`Error ID: ${this.state.errorId}\n\nMessage: ${this.state.error.message}\n`)}`;
      return (
        <div className="error-banner">
          <strong>Something went wrong</strong>
          <p>{this.state.error.message}</p>
          <p className="muted">Error ID: {this.state.errorId}</p>
          <div className="error-banner-actions">
            <button onClick={() => this.setState({ error: null, errorId: '' })}>Retry</button>
            <a className="btn-secondary" href={reportURL}>
              Report issue
            </a>
            <a className="btn-secondary" href="mailto:support@nexus.io">
              Contact support
            </a>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}

function buildErrorID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `err-${Date.now()}`;
}
