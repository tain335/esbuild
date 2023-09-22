// import React, {useState} from 'react'
import { test } from './moduleA';
// import "./email-validator"

// function Component() {
//   const [state, setState] = useState(0)
//   return <div>
//     <h1>ddd</h1>
//   </div>
// }

function product(numberA: number) {
  return numberA * numberA
}

function sum(numberA: number, numberB: number) {
  return product(numberA) + numberB
}

const apple = 10;
const orange = 20;
const total = sum(apple, orange)

console.log(test())
console.log(total);
// console.log(render())