"use client"

import { format } from "date-fns"
import { CalendarIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Calendar } from "@/components/ui/calendar"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"

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
  return (
    <div className="flex space-x-4 mb-6">
      <Button
        variant={statusFilter === "pending" ? "default" : "outline"}
        onClick={() =>
          onStatusChange(statusFilter === "pending" ? null : "pending")
        }
      >
        Pending
      </Button>
      <Button
        variant={statusFilter === "completed" ? "default" : "outline"}
        onClick={() =>
          onStatusChange(statusFilter === "completed" ? null : "completed")
        }
      >
        Completed
      </Button>

      <Popover>
        <PopoverTrigger asChild>
          <Button variant="outline">
            <CalendarIcon className="mr-2 h-4 w-4" />
            {selectedDate ? format(selectedDate, "PPP") : "Select Date"}
          </Button>
        </PopoverTrigger>
        <PopoverContent align="start">
          <Calendar
            mode="single"
            selected={selectedDate || undefined}
            onSelect={onDateChange}
          />
        </PopoverContent>
      </Popover>

      <Button variant="destructive" onClick={onReset}>
        Reset
      </Button>
    </div>
  )
}
