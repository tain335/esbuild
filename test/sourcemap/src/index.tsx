import React, {useState} from 'react'
import { test } from './moduleA';
function Component() {
  const [state, setState] = useState(0)
  return <div>
    <h1>ddd</h1>
  </div>
}

function sum(numberA: number, numberB: number) {
  return numberA + numberB
}

const apple = 10;
const orange = 20;
const total = sum(apple, orange)

console.log(total);
// console.log(render())
console.log(test())