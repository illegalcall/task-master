"use client"

import { format } from "date-fns"
import { CalendarIcon, FilterIcon, XCircleIcon } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Calendar } from "@/components/ui/calendar"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"

interface JobsFilterProps {
  statusFilter: string | null
  selectedDate: Date | null
  onStatusChange: (status: string | null) => void
  onDateChange: (date: Date | undefined) => void
  onReset: () => void
}

export function JobsFilter({
  statusFilter,
  selectedDate,
  onStatusChange,
  onDateChange,
  onReset,
}: JobsFilterProps) {
  const activeFilters = (statusFilter ? 1 : 0) + (selectedDate ? 1 : 0)

  return (
    <div className="mb-8 rounded-lg border bg-card p-4">
      <div className="mb-4 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <FilterIcon className="h-5 w-5" />
          <h2 className="font-semibold">Filter Jobs</h2>
          {activeFilters > 0 && (
            <Badge variant="secondary" className="ml-2">
              {activeFilters} active
            </Badge>
          )}
        </div>
        {activeFilters > 0 && (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-8 px-2 text-muted-foreground"
                  onClick={onReset}
                >
                  <XCircleIcon className="h-4 w-4 mr-1" />
                  Clear filters
                </Button>
              </TooltipTrigger>
              <TooltipContent>Reset all filters</TooltipContent>
            </Tooltip>
          </TooltipProvider>
        )}
      </div>

      <div className="flex flex-wrap gap-4">
        <div className="flex flex-col gap-1.5">
          <label className="text-sm font-medium">Status</label>
          <Select
            value={statusFilter || "all"}
            onValueChange={(value) =>
              onStatusChange(value === "all" ? null : value)
            }
          >
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="All statuses" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All statuses</SelectItem>
              <SelectItem value="pending">Pending</SelectItem>
              <SelectItem value="completed">Completed</SelectItem>
              <SelectItem value="failed">Failed</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-1.5">
          <label className="text-sm font-medium">Date</label>
          <Popover>
            <PopoverTrigger asChild>
              <Button
                variant="outline"
                className={selectedDate ? "font-medium" : ""}
              >
                <CalendarIcon className="mr-2 h-4 w-4" />
                {selectedDate ? format(selectedDate, "PPP") : "All dates"}
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-auto p-0" align="start">
              <Calendar
                mode="single"
                selected={selectedDate || undefined}
                onSelect={(date) => {
                  onDateChange(date)
                }}
                initialFocus
              />
            </PopoverContent>
          </Popover>
        </div>
      </div>
    </div>
  )
}
