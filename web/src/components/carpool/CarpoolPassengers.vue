<script setup lang="ts">
import NumberInput from '@/components/NumberInput.vue'
import { Users, Plus, Trash2, Calculator, ChevronDown, ChevronUp } from 'lucide-vue-next'
import { cents, euros, newPassenger, type LegForm, type PassengerForm } from '@/utils/carpool'
import { currencySymbol, formatAmount } from '@/currency'
import { formatDistance } from '@/units'

// The passengers of a carpool: where each boards and alights, the seats, what was paid against the fair share, and the
// section by section calculation of that share on demand (expanded is the index of the passenger whose calculation is open).
const props = defineProps<{ legs: LegForm[]; stops: string[]; live: any; currency: string }>()
const passengers = defineModel<PassengerForm[]>('passengers', { required: true })
const expandedPassengerIndex = defineModel<number | null>('expanded', { required: true })
const fmt = (v: number) => formatAmount(Number(v || 0), props.currency)

function togglePassengerMath(index: number) {
  expandedPassengerIndex.value = expandedPassengerIndex.value === index ? null : index
}

function addPassenger() {
  passengers.value.push(newPassenger(passengers.value.length, props.legs.length))
}

function removePassenger(index: number) {
  passengers.value.splice(index, 1)
}

function onBoardChange(p: PassengerForm) {
  if (p.alight_stop_index <= p.board_stop_index) p.alight_stop_index = p.board_stop_index + 1
}

function applyFairPrice(index: number) {
  passengers.value[index].amount_paid = euros(props.live.shares[index])
}
</script>

<template>
  <div class="space-y-2">
    <div class="flex items-center justify-between">
      <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
        <Users class="w-4 h-4 text-blue-400" />
        {{ $t('carpool.carpoolTripModal.passengersBoardingAndAlighting') }}
      </h4>
      <button type="button" @click="addPassenger" class="text-xs text-blue-400 hover:text-blue-300 font-semibold flex items-center gap-1">
        <Plus class="w-3.5 h-3.5" /> {{ $t('carpool.carpoolTripModal.addAPassenger') }}
      </button>
    </div>

    <div v-for="(p, index) in passengers" :key="index" class="bg-slate-950/50 border border-slate-800 rounded-xl p-3 space-y-2">
      <div class="grid grid-cols-2 sm:grid-cols-12 gap-2 items-end text-xs">
        <div class="col-span-2 sm:col-span-3">
          <label :for="`passenger-name-${index}`" class="block text-xs text-slate-400 mb-0.5">{{ $t('carpool.carpoolTripModal.name') }}</label>
          <input
            :id="`passenger-name-${index}`"
            v-model="p.passenger_name"
            class="field"
          />
        </div>
        <div class="sm:col-span-3">
          <label :for="`passenger-board-${index}`" class="block text-xs text-slate-400 mb-0.5">{{ $t('carpool.carpoolTripModal.getsOnAt') }}</label>
          <select
            :id="`passenger-board-${index}`"
            v-model.number="p.board_stop_index"
            @change="onBoardChange(p)"
            class="field"
          >
            <option v-for="(name, s) in stops.slice(0, -1)" :key="s" :value="s">{{ name }}</option>
          </select>
        </div>
        <div class="sm:col-span-3">
          <label :for="`passenger-alight-${index}`" class="block text-xs text-slate-400 mb-0.5">{{ $t('carpool.carpoolTripModal.getsOffAt') }}</label>
          <select
            :id="`passenger-alight-${index}`"
            v-model.number="p.alight_stop_index"
            class="field"
          >
            <option v-for="(name, s) in stops" v-show="s > p.board_stop_index" :key="s" :value="s" :disabled="s <= p.board_stop_index">{{ name }}</option>
          </select>
        </div>
        <div>
          <label :for="`passenger-seats-${index}`" class="block text-xs text-slate-400 mb-0.5">{{ $t('carpool.carpoolTripModal.seats') }}</label>
          <input
            :id="`passenger-seats-${index}`"
            v-model.number="p.seats"
            type="number"
            min="1"
            max="7"
            class="field"
          />
        </div>
        <div class="sm:col-span-2">
          <label :for="`passenger-paid-${index}`" class="block text-xs text-slate-400 mb-0.5">{{ $t('carpool.carpoolTripModal.paid', { cur: currencySymbol(currency) }) }}</label>
          <NumberInput
            :id="`passenger-paid-${index}`"
            v-model="p.amount_paid"
            min="0"
            class="field font-bold text-success-400"
          />
        </div>
      </div>
      <div class="flex flex-wrap items-center justify-between gap-2 text-xs">
        <span class="text-slate-400">
          {{ $t('carpool.carpoolTripModal.fairShare') }} <strong class="text-slate-200">{{ fmt(euros(live.shares[index] || 0)) }}</strong>
          <span :class="cents(p.amount_paid) >= (live.shares[index] || 0) ? 'text-success-400' : 'text-warning-400'" class="ml-2">
            {{ cents(p.amount_paid) >= (live.shares[index] || 0)
              ? $t('carpool.above', { amount: fmt(euros(cents(p.amount_paid) - (live.shares[index] || 0))) })
              : $t('carpool.below', { amount: fmt(euros((live.shares[index] || 0) - cents(p.amount_paid))) }) }}
          </span>
        </span>
        <span class="flex items-center gap-3">
          <button
            type="button"
            @click="togglePassengerMath(index)"
            class="text-sky-400 hover:text-sky-300 font-medium flex items-center gap-1 transition-colors"
            :title="expandedPassengerIndex === index ? $t('carpool.carpoolTripModal.hideCalcDetail') : $t('carpool.carpoolTripModal.explainShare')"
          >
            <Calculator class="w-3.5 h-3.5" />
            <span>{{ expandedPassengerIndex === index ? $t('carpool.carpoolTripModal.hideCalc') : $t('carpool.carpoolTripModal.calcDetail') }}</span>
            <ChevronUp v-if="expandedPassengerIndex === index" class="w-3.5 h-3.5" />
            <ChevronDown v-else class="w-3.5 h-3.5" />
          </button>
          <button type="button" @click="applyFairPrice(index)" class="text-sky-400 hover:text-sky-300 font-semibold">{{ $t('carpool.carpoolTripModal.applyTheFairShare') }}</button>
          <button v-if="passengers.length > 1" type="button" @click="removePassenger(index)" class="text-slate-400 hover:text-danger-400" :title="$t('carpool.carpoolTripModal.removeThisPassenger')">
            <Trash2 class="w-3.5 h-3.5" />
          </button>
        </span>
      </div>

      <!-- Mathematical Breakdown Card -->
      <div
        v-if="expandedPassengerIndex === index"
        class="mt-3 pt-3 border-t border-slate-800 space-y-2.5 bg-slate-950/70 p-3 rounded-xl"
      >
        <div class="flex items-center justify-between text-xs">
          <span class="font-bold text-slate-200 flex items-center gap-1.5">
            <Calculator class="w-3.5 h-3.5 text-sky-400" />
            {{ $t('carpool.carpoolTripModal.formulaPerSection', { name: p.passenger_name || $t('carpool.passenger', { n: index + 1 }) }) }}
          </span>
          <span class="text-xs text-slate-400 font-medium">
            {{ $t('carpool.carpoolTripModal.seatsBooked', p.seats) }}
          </span>
        </div>

        <div class="space-y-2">
          <div
            v-for="(leg, legIdx) in legs"
            :key="legIdx"
            class="text-xs p-2.5 rounded-lg border transition-colors"
            :class="p.board_stop_index <= legIdx && legIdx < p.alight_stop_index
              ? 'bg-slate-900/90 border-sky-500/30 text-slate-200'
              : 'bg-slate-900/30 border-slate-800/50 text-slate-400 opacity-60'"
          >
            <div class="flex items-center justify-between font-semibold">
              <span class="flex items-center gap-1.5">
                <span class="w-4 h-4 rounded-full bg-slate-800 flex items-center justify-center text-xs font-mono text-slate-300">
                  {{ legIdx + 1 }}
                </span>
                <span>{{ stops[legIdx] }} → {{ stops[legIdx + 1] }}</span>
                <span v-if="Number(leg.distance_km)" class="text-slate-400 font-normal">({{ formatDistance(Number(leg.distance_km), 1) }})</span>
              </span>
              <span v-if="p.board_stop_index <= legIdx && legIdx < p.alight_stop_index" class="text-sky-300 font-bold">
                {{ fmt(euros((live.legDetails[legIdx]?.perPerson || 0) * (Number(p.seats) || 1))) }}
              </span>
              <span v-else class="text-slate-400 italic text-xs">
                {{ $t('carpool.carpoolTripModal.notTravelled') }}
              </span>
            </div>

            <div v-if="p.board_stop_index <= legIdx && legIdx < p.alight_stop_index" class="mt-1.5 pl-5.5 text-xs text-slate-400 flex flex-wrap items-center gap-x-2.5 gap-y-1">
              <span>{{ $t('carpool.carpoolTripModal.actualCostOfTheSection') }} <strong class="text-slate-200">{{ fmt(euros(live.legDetails[legIdx]?.total || 0)) }}</strong></span>
              <span>•</span>
              <span>{{ $t('carpool.carpoolTripModal.occupants') }} <strong class="text-slate-200">{{ $t('carpool.carpoolTripModal.oneDriverPlusPassengers', { legDetails: live.legDetails[legIdx]?.seats || 0, legDetails2: 1 + (live.legDetails[legIdx]?.seats || 0) }) }}</strong></span>
              <span>•</span>
              <span class="text-sky-300/90">
                {{ $t('carpool.carpoolTripModal.calcFormula', { total: fmt(euros(live.legDetails[legIdx]?.total || 0)), people: 1 + (live.legDetails[legIdx]?.seats || 0) }) }}{{ p.seats > 1 ? $t('carpool.carpoolTripModal.calcSeats', { seats: p.seats }) : '' }}
              </span>
            </div>
          </div>
        </div>

        <div class="pt-2 border-t border-slate-800/80 flex items-center justify-between text-xs">
          <span class="text-slate-400">{{ $t('carpool.carpoolTripModal.totalFairShareDue') }}</span>
          <div class="text-right">
            <span class="font-bold text-success-400 text-sm">{{ fmt(euros(live.shares[index] || 0)) }}</span>
            <span class="text-xs text-slate-400 block">{{ $t('carpool.carpoolTripModal.exactSumOfTheSections') }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
