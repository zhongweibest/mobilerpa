export interface PaginationQuery {
  page: number;
  page_size: number;
  slot_zone?: string;
  slot_row?: string;
  slot_position?: string;
  target_type?: string;
  schedule_type?: string;
  plan_name?: string;
  plan_def_id?: string;
}

export interface PaginatedResult<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}
