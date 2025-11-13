export interface CapturedRequest {
  id: number;
  timestamp: string;
  method: string;
  url: string;
  request_headers: Record<string, string[]>;
  request_body: string;
  response_status: number;
  response_headers: Record<string, string[]>;
  response_body: string;
  bookmarked?: boolean;
  note?: string;
}
