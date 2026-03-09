export interface Pagination {
  count: number;
  page_size: number;
  next_cursor?: string;
  previous_cursor?: string;
}
