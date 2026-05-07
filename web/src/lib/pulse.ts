export interface PulseInput {
  kind: 'success' | 'check' | 'error' | 'skip';
  count: number;
}
